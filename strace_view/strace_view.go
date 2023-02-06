package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hugelgupf/go-strace/strace"
	"github.com/iimos/play/strace_view/syscalls"
)

//go:embed template.html
var templateHTML string

//go:embed script.js
var scriptJS string

//go:embed style.css
var styleCSS string

//go:embed syscalls.json
var syscallsJSON string

func main_v1() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <path to file>\n", os.Args[0])
		os.Exit(1)
	}
	path := os.Args[1]

	events, err := getTracesFromFile(path)
	if err != nil {
		panic(err)
	}

	outpath := "strace.html"
	out, err := NewHTMLWriter(outpath)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	jsonEvents, err := json.Marshal(events)
	if err != nil {
		panic(err)
	}

	html := strings.Replace(templateHTML, "{{events}}", string(jsonEvents), 1)
	html = strings.Replace(html, "{{js}}", scriptJS, 1)
	html = strings.Replace(html, "{{css}}", scriptJS, 1)
	html = strings.Replace(html, "{{syscalls}}", syscallsJSON, 1)
	err = out.WriteString(html)
	if err != nil {
		panic(err)
	}

	fmt.Printf("result written in %q\n", outpath)
}

// go run -ldflags "-X main.debug=1" .
var debug = ""

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s PROG [ARGS]\n", os.Args[0])
		os.Exit(1)
	}

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	events, err := trace(cmd)
	if err != nil {
		panic(err)
	}

	outpath := "strace.html"
	out, err := NewHTMLWriter(outpath)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	jsonEvents, err := json.Marshal(events)
	if err != nil {
		panic(err)
	}

	html := strings.Replace(templateHTML, "{{events}}", string(jsonEvents), 1)
	html = strings.Replace(html, "{{js}}", scriptJS, 1)
	html = strings.Replace(html, "{{css}}", scriptJS, 1)
	html = strings.Replace(html, "{{syscalls}}", syscallsJSON, 1)
	err = out.WriteString(html)
	if err != nil {
		panic(err)
	}

	fmt.Printf("result written in %q\n", outpath)
}

func trace(cmd *exec.Cmd) (TraceEvents, error) {
	events := TraceEvents{
		Event:           make([]Event, 0, 64),
		DisplayTimeUnit: "ns",
	}
	err := strace.Trace(cmd, func(t strace.Task, record *strace.TraceRecord) error {
		switch record.Event {
		case strace.SyscallExit:
			e := newTraceEvent(t, record)
			events.Event = append(events.Event, e)
			if debug != "" {
				fmt.Printf("%#v\n", e)
			}

		case strace.SignalExit:
			if debug != "" {
				fmt.Printf("PID %d exited from signal %s\n", record.PID, syscalls.SignalString(record.SignalExit.Signal))
			}
		case strace.Exit:
			if debug != "" {
				fmt.Printf("PID %d exited from exit status %d (code = %d)\n", record.PID, record.Exit.WaitStatus, record.Exit.WaitStatus.ExitStatus())
			}
		case strace.SignalStop:
			if debug != "" {
				fmt.Printf("PID %d got signal %s\n", record.PID, syscalls.SignalString(record.SignalStop.Signal))
			}
		case strace.NewChild:
			if debug != "" {
				fmt.Printf("PID %d spawned new child %d\n", record.PID, record.NewChild.PID)
			}
		}
		return nil
	})
	return events, err
}

const LogMaximumSize = 1024

func newTraceEvent(t strace.Task, record *strace.TraceRecord) Event {
	call := record.Syscall
	syscallInfo := syscalls.Details(call)
	e := Event{
		Name:      syscallInfo.Name,
		Cat:       "successful",
		Ph:        "X", // Complete event
		PID:       record.PID,
		TID:       record.PID,
		Timestamp: int(time.Now().UnixNano()),
		Duration:  int(call.Duration.Nanoseconds()),
		Args: Args{
			// FullLine:    syscalls.SyscallString(syscallInfo, t, call.Args, call.Ret[0], call.Errno),
			Syscall: syscallInfo.Name,
		},
	}

	if call.Errno == 0 {
		if call.Ret[0].Value < 128 {
			e.Args.Result = strconv.Itoa(int(call.Ret[0].Int()))
		} else {
			e.Args.Result = fmt.Sprintf("%#x", call.Ret[0].Uint64())
		}
	} else {
		e.Args.Result = fmt.Sprintf("%q (%d)", call.Errno, call.Errno)
		e.Cat = "failed"
	}

	args := syscalls.ArgumentsStrings(syscallInfo, t, call.Args, call.Ret[0], LogMaximumSize)
	e.Args.SyscallArgs = strings.Join(args, ", ")

	return e
}

func getTracesFromFile(filepath string) (TraceEvents, error) {
	f, err := os.OpenFile(filepath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return TraceEvents{}, err
	}
	defer f.Close()

	events := TraceEvents{
		Event:           make([]Event, 0, 64),
		DisplayTimeUnit: "ns",
	}

	p := NewStraceParser()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		e, complete, err := p.ParseLine(line)
		if err != nil {
			fmt.Printf("%s: %s\n", err, line)
			continue
		}
		if complete {
			events.Event = append(events.Event, e)
		}
	}
	if err := sc.Err(); err != nil {
		return events, err
	}
	return events, nil
}
