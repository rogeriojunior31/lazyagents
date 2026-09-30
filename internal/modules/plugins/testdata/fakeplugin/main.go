// Command fakeplugin is the plugin the tests run on every OS, in place of shell
// scripts Windows cannot execute. FAKEPLUGIN_MODE picks the behavior; the host
// passes its environment to plugins, so tests set it with t.Setenv.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"
)

type msg struct {
	Type string `json:"type"`
	Key  string `json:"key"`
	Code int    `json:"code"`
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "exec-child": // what the echo mode asks the host to run
			fmt.Println("out")
			os.Exit(2)
		case "sleep-child": // a long exec, to check it stops with the service
			time.Sleep(30 * time.Second)
			return
		}
	}
	mode := os.Getenv("FAKEPLUGIN_MODE")
	if len(args) == 0 || args[0] != "serve" {
		cli(mode, args)
		return
	}
	switch mode {
	case "good":
		serve(`{"type":"manifest","title":"  Hello  ","help":[{"title":"Keys","keys":[["x","echo"]]}],"commands":[{"name":"ping","desc":"pings"},{"name":"Bad Name","desc":"x"}]}`,
			`{"type":"frame","view":"hello","count":2}`,
			func(m msg) bool {
				switch m.Type {
				case "key":
					emit(`{"type":"frame","view":` + quote("key "+m.Key) + `}`)
				case "reload":
					fmt.Fprintln(os.Stderr, "reload!")
					emit(`{"type":"frame","view":"reloaded"}`)
				}
				return true
			})
	case "echo":
		self, _ := os.Executable()
		serve(`{"type":"manifest","title":"Echo","help":[{"title":"Echo","keys":[["x","echo"]]}],"commands":[{"name":"ping","desc":"pings"}]}`,
			`{"type":"frame","view":"\u001b[1mhello ✓\u001b[0m\u001b[2J","count":3}`,
			func(m msg) bool {
				switch m.Type {
				case "key":
					emit(`{"type":"frame","view":` + quote("key "+m.Key) + `,"capturing":true}`)
				case "command":
					emit(`{"type":"exec","execId":7,"argv":[` + quote(self) + `,"exec-child"]}`)
				case "exec_result":
					emit(`{"type":"frame","view":"exec code ` + strconv.Itoa(m.Code) + `"}`)
				case "reload":
					return false // exits 0: the tab goes to its dead state
				}
				return true
			})
	case "spawn": // starts a child that would outlive it, to check the tree stops
		spawnChild()
		serve(`{"type":"manifest","title":"Spawn"}`, "", func(msg) bool { return true })
	case "fail":
		fmt.Fprintln(os.Stderr, "failed")
		os.Exit(1)
	default: // "hi"
		serve(`{"type":"manifest","title":"Hi"}`, "", func(msg) bool { return true })
	}
}

// cli is the pass-through mode (`lazyagents <id> args`).
func cli(mode string, args []string) {
	switch mode {
	case "hang": // a doctor that never ends, with a child of its own
		spawnChild()
		time.Sleep(30 * time.Second)
	case "cli":
		first := ""
		if len(args) > 0 {
			first = args[0]
		}
		fmt.Println(first + " " + os.Getenv("LAZYAGENTS_DATA_DIR"))
		os.Exit(7)
	default:
		code, _ := strconv.Atoi(os.Getenv("FAKEPLUGIN_EXIT"))
		os.Exit(code)
	}
}

// serve reads init, answers manifest and a first frame, then hands every
// message to on until it returns false or stdin closes.
func serve(manifest, frame string, on func(msg) bool) {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	if !in.Scan() {
		return
	}
	emit(manifest)
	if frame != "" {
		emit(frame)
	}
	for in.Scan() {
		var m msg
		if json.Unmarshal(in.Bytes(), &m) == nil && !on(m) {
			return
		}
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
}

// spawnChild starts `self sleep-child` and writes its pid to FAKEPLUGIN_PIDFILE.
func spawnChild() {
	self, _ := os.Executable()
	child := exec.Command(self, "sleep-child")
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = os.WriteFile(os.Getenv("FAKEPLUGIN_PIDFILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o644)
}

func emit(line string) { fmt.Println(line) }

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
