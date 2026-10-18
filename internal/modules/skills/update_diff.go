package skills

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

func readContext(ctx context.Context, path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(contextReader{ctx, f})
}

func visibleFiles(ctx context.Context, root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, contextReader{ctx, f})
		f.Close()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = fmt.Sprintf("%x", h.Sum(nil))
		return nil
	})
	return out, err
}

// The preview bounds files, bytes and lines. Hash comparison still covers all
// visible regular files; binary/large content is reported without loading it.
func skillDiff(ctx context.Context, local, remote string) ([]string, string, error) {
	before, err := visibleFiles(ctx, local)
	if err != nil {
		return nil, "", err
	}
	after, err := visibleFiles(ctx, remote)
	if err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(before)+len(after))
	for n := range before {
		names = append(names, n)
	}
	for n := range after {
		if _, ok := before[n]; !ok {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	var files []string
	var diff strings.Builder
	changed := 0
	for _, name := range names {
		if before[name] == after[name] {
			continue
		}
		changed++
		if changed > 200 {
			continue
		}
		kind := "Modified"
		if _, ok := before[name]; !ok {
			kind = "Added"
		}
		if _, ok := after[name]; !ok {
			kind = "Removed"
		}
		label := kind + ": " + reviewText(name)
		files = append(files, label)
		if diff.Len() >= 24<<10 {
			continue
		}
		fmt.Fprintf(&diff, "\n--- local/%s\n+++ upstream/%s\n", reviewText(name), reviewText(name))
		old, oldOK, err := previewFile(ctx, local, name, before)
		if err != nil {
			return nil, "", err
		}
		next, nextOK, err := previewFile(ctx, remote, name, after)
		if err != nil {
			return nil, "", err
		}
		if !oldOK || !nextOK {
			diff.WriteString("Binary, invalid UTF-8 or file over 8 KiB; content diff omitted.\n")
			continue
		}
		a, b := strings.Split(old, "\n"), strings.Split(next, "\n")
		prefix := 0
		for prefix < min(len(a), len(b)) && a[prefix] == b[prefix] {
			prefix++
		}
		tail := 0
		for tail < min(len(a)-prefix, len(b)-prefix) && a[len(a)-tail-1] == b[len(b)-tail-1] {
			tail++
		}
		fmt.Fprintf(&diff, "@@ local line %d / upstream line %d @@\n", prefix+1, prefix+1)
		lines := 0
		for i, block := range [][]string{a[prefix : len(a)-tail], b[prefix : len(b)-tail]} {
			sign := []string{"-", "+"}[i]
			for _, line := range block {
				if lines >= 80 || diff.Len() >= 24<<10 {
					break
				}
				text := reviewText(line)
				if len(text) > 512 {
					text = string([]rune(text)[:min(128, len([]rune(text)))]) + " [line truncated]"
				}
				diff.WriteString(sign + text + "\n")
				lines++
			}
		}
		if lines < len(a)+len(b)-2*prefix-2*tail {
			diff.WriteString("[file diff truncated]\n")
		}
	}
	if changed > 200 {
		files = append(files, fmt.Sprintf("%d further changed files omitted", changed-200))
	}
	if diff.Len() >= 24<<10 {
		diff.WriteString("\n[diff truncated at 24 KiB; file list above may contain further changes]\n")
	}
	return files, diff.String(), nil
}

func previewFile(ctx context.Context, root, name string, files map[string]string) (string, bool, error) {
	if _, ok := files[name]; !ok {
		return "", true, nil
	}
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(contextReader{ctx, f}, (8<<10)+1))
	if err != nil {
		return "", false, err
	}
	return string(b), len(b) <= 8<<10 && utf8.Valid(b) && !slices.Contains(b, byte(0)), nil
}

func reviewText(text string) string {
	var b strings.Builder
	for _, r := range text {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.In(r, unicode.Cf) {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
