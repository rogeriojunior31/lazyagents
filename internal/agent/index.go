package agent

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Incremental index of JSONL transcripts.
//
// Agent transcripts only grow. Per file, the index keeps how far it read and
// what it extracted (preview, token sums, usage events, limits): an unchanged
// file costs a stat, a grown one is read from the offset, and a rewritten one
// (shrank or changed its head) is read again. It is a cache: deleting it only
// costs one full reread. It never stores conversation text beyond the title.

// indexVersion changes whenever indexEntry changes shape; an old index is dropped.
const indexVersion = 4

// headLen is how many leading bytes identify a file's content.
const headLen = 256

// usageSlot buckets responses in 15-minute slots: every real time zone is a
// multiple of 15 min, so a slot never crosses local midnight and the 5h block
// starts at the exact time of its first response.
const usageSlot = 15 * time.Minute

// usageBucket sums the responses of one slot, model and cwd. Kept compact on
// purpose: the index holds thousands of them.
type usageBucket struct {
	First                          int64  // unix nano of the slot's first response (0 = no timestamp)
	M                              int    // index into indexEntry.Models
	CWD                            string // "" = indexEntry.CWD; noCWD = the line had none
	In, Out, CacheRead, CacheWrite int
	Cost                           float64 // USD recorded by the agent (Pi)
	N                              int     // responses summed
}

// noCWD marks a response whose line had no cwd (the session's applies). Paths
// never contain NUL, so it cannot collide with a real dir.
const noCWD = "\x00"

func slotOf(first int64) int64 {
	if first == 0 {
		return -1
	}
	return first / int64(usageSlot)
}

// indexEntry is what the index knows about one file.
type indexEntry struct {
	Size, ModTime, Offset int64
	HeadLen               int
	Head                  uint64

	// preview: session cwd (first seen), first prompt, title
	CWD, FirstPrompt, AITitle string
	// token sums (Claude)
	Usage    Usage
	HasUsage bool
	// bucketed responses; Legacy holds old Codex token_count events, used only
	// when there is no token_usage_record
	Events, Legacy []usageBucket
	Models         []string
	// current rollout context (Codex): inherited model and cwd
	Model, CtxCWD string
	// last recorded limits (Codex)
	Rate   *codexRateLimits
	RateAt int64
}

// addEvent adds a response to its bucket in list (Events or Legacy).
func (e *indexEntry) addEvent(list *[]usageBucket, ts time.Time, model, cwd string, u Usage) {
	first := int64(0)
	if !ts.IsZero() {
		first = ts.UnixNano()
	}
	switch cwd {
	case "":
		cwd = noCWD
	case e.CWD:
		cwd = ""
	}
	m := slices.Index(e.Models, model)
	if m < 0 {
		e.Models = append(e.Models, model)
		m = len(e.Models) - 1
	}
	slot := slotOf(first)
	// responses arrive in order: an existing bucket is the last one
	for i := len(*list) - 1; i >= 0 && slotOf((*list)[i].First) == slot; i-- {
		b := &(*list)[i]
		if b.M == m && b.CWD == cwd {
			b.In += u.Input
			b.Out += u.Output
			b.CacheRead += u.CacheRead
			b.CacheWrite += u.CacheWrite
			b.Cost += u.Cost
			b.N++
			return
		}
	}
	*list = append(*list, usageBucket{First: first, M: m, CWD: cwd,
		In: u.Input, Out: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, Cost: u.Cost, N: 1})
}

// events converts buckets into UsageEvents; missing time or cwd use the session's.
func (e indexEntry) events(list []usageBucket, s Session) []UsageEvent {
	out := make([]UsageEvent, 0, len(list))
	for _, b := range list {
		ts := s.MTime
		if b.First != 0 {
			ts = time.Unix(0, b.First)
		}
		cwd := b.CWD
		switch cwd {
		case "":
			cwd = e.CWD
		case noCWD:
			cwd = s.CWD
		}
		model := ""
		if b.M < len(e.Models) {
			model = e.Models[b.M]
		}
		out = append(out, UsageEvent{Time: ts, Model: model, CWD: cwd, N: b.N, Usage: Usage{
			Input: b.In, Output: b.Out, CacheRead: b.CacheRead, CacheWrite: b.CacheWrite, Cost: b.Cost, Model: model}})
	}
	return out
}

// lineScanner extracts what an adapter needs from one line.
type lineScanner func(e *indexEntry, line []byte)

// Index is shared by the adapters. path "" = memory only.
type Index struct {
	path    string
	mu      sync.Mutex
	entries map[string]*indexEntry
	dirty   bool
}

// NewIndex opens the index at path (missing, unreadable or another version = empty).
func NewIndex(path string) *Index {
	x := &Index{path: path, entries: map[string]*indexEntry{}}
	if path == "" {
		return x
	}
	f, err := os.Open(path)
	if err != nil {
		return x
	}
	defer f.Close()
	var disk struct {
		Version int
		Entries map[string]*indexEntry
	}
	if gob.NewDecoder(f).Decode(&disk) == nil && disk.Version == indexVersion && disk.Entries != nil {
		x.entries = disk.Entries
	}
	return x
}

// save writes the index if something changed. Failure is not a user error: it is a cache.
func (x *Index) save() {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.path == "" || !x.dirty {
		return
	}
	var buf bytes.Buffer
	disk := struct {
		Version int
		Entries map[string]*indexEntry
	}{indexVersion, x.entries}
	if gob.NewEncoder(&buf).Encode(disk) != nil {
		return
	}
	if fsutil.WriteAtomic(x.path, buf.Bytes(), 0o600) == nil {
		x.dirty = false
	}
}

// retain forgets files under prefix that are not in keep (deleted sessions), so
// the index does not grow forever.
func (x *Index) retain(prefix string, keep map[string]bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	for p := range x.entries {
		if strings.HasPrefix(p, prefix) && !keep[p] {
			delete(x.entries, p)
			x.dirty = true
		}
	}
}

func (x *Index) get(path string) (indexEntry, bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	e, ok := x.entries[path]
	if !ok {
		return indexEntry{}, false
	}
	return *e, true
}

// refresh brings path's entry up to date with the file and returns it.
func (x *Index) refresh(path string, scan lineScanner) (indexEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return indexEntry{}, fmt.Errorf("reading %s: %w", path, err)
	}
	old, known := x.get(path)
	if known && old.Size == info.Size() && old.ModTime == info.ModTime().UnixNano() {
		return old, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return indexEntry{}, fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()
	e := indexEntry{}
	if known && info.Size() >= old.Offset && old.HeadLen > 0 && hashAt(f, old.HeadLen) == old.Head {
		// only grew: continue from the offset, keeping what was read
		e = old
		e.Events, e.Legacy, e.Models = slices.Clone(old.Events), slices.Clone(old.Legacy), slices.Clone(old.Models)
	}
	e.HeadLen = int(min(headLen, info.Size()))
	e.Head = hashAt(f, e.HeadLen)
	if _, err := f.Seek(e.Offset, io.SeekStart); err != nil {
		return indexEntry{}, fmt.Errorf("reading %s: %w", path, err)
	}
	e.Offset += scanLines(f, func(line []byte) { scan(&e, line) })
	e.Size, e.ModTime = info.Size(), info.ModTime().UnixNano()

	x.mu.Lock()
	x.entries[path] = &e
	x.dirty = true
	x.mu.Unlock()
	return e, nil
}

// refreshAll refreshes many files in parallel (the first load reads the whole
// history; one worker per CPU).
func (x *Index) refreshAll(paths []string, scan lineScanner) {
	work := make(chan string)
	var wg sync.WaitGroup
	for range max(1, min(runtime.NumCPU(), len(paths))) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range work {
				_, _ = x.refresh(p, scan) // unreadable: the session has no preview
			}
		}()
	}
	for _, p := range paths {
		work <- p
	}
	close(work)
	wg.Wait()
}

// hashAt hashes the first n bytes of the file (0 if unreadable).
func hashAt(f *os.File, n int) uint64 {
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, 0); err != nil && !errors.Is(err, io.EOF) {
		return 0
	}
	h := fnv.New64a()
	h.Write(buf)
	return h.Sum64()
}

// scanLines yields each complete line of r and returns the bytes consumed. A
// final line without newline (agent still writing) counts only if it is
// already valid JSON; otherwise it waits for the next read. A line over
// maxLineBuf stops the read there.
func scanLines(r io.Reader, fn func([]byte)) int64 {
	var consumed, pending int64
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	sc.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			pending = int64(i + 1)
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 && json.Valid(data) {
			pending = int64(len(data))
			return len(data), data, bufio.ErrFinalToken
		}
		if atEOF {
			return 0, nil, bufio.ErrFinalToken
		}
		return 0, nil, nil
	})
	for sc.Scan() {
		consumed += pending
		pending = 0
		if line := sc.Bytes(); len(line) > 0 {
			fn(line)
		}
	}
	return consumed
}
