package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// taken from https://github.com/lbirchler/strace-perfetto/blob/main/events.go
	reSuccessful = `^(\d+) +(\d+\.\d+) +(\w+)+(\(.*?\)) +\= (\d.*?)( <[0-9\.]+>)?$`             // pid,ts,syscall,args,returnValue,duration
	reFailed     = `^(\d+) +(\d+\.\d+) +(\w+)+(\(.*?\)) +\= (\-\d.*?)( <[0-9\.]+>)?$`           // pid,ts,syscall,args,returnValue,duration
	reUnfinished = `^(\d+) +(\d+\.\d+) +(\w+)+(\(.+)<unfinished ...>`                           // pid,ts,syscall,args
	reDetached   = `^(\d+) +(\d+\.\d+) <\.\.\. +(\w+) resumed>(.*?\)) +\= (.+?)( <[0-9\.]+>)?$` // pid,ts,syscall,args,returnValue,duration
	reEtc        = `^(\d+) +(\d+\.\d+) (.*)`                                                    // pid,ts
)

var regexes = []struct {
	category string
	re       *regexp.Regexp
}{
	{"successful", regexp.MustCompile(reSuccessful)},
	{"failed", regexp.MustCompile(reFailed)},
	{"unfinished", regexp.MustCompile(reUnfinished)},
	{"detached", regexp.MustCompile(reDetached)},
	{"etc", regexp.MustCompile(reEtc)},
}

// Format https://docs.google.com/document/d/1CvAClvFfyA5R-PhYUmn5OOQtYMH4h6I0nSsKchNAySU/preview
type Event struct {
	Name      string `json:"name"`
	Cat       string `json:"cat"`
	Ph        string `json:"ph"`
	PID       int    `json:"pid"`
	TID       int    `json:"tid"`
	Timestamp int    `json:"ts"`
	Duration  int    `json:"dur,omitempty"`
	Args      Args   `json:"args"`
}

type Args struct {
	FullLine    string
	Syscall     string
	SyscallArgs string
	Result      string
	Duration    float64
}

type StraceParser struct {
	prevUnfinished map[int]Event
}

func NewStraceParser() *StraceParser {
	return &StraceParser{
		prevUnfinished: make(map[int]Event),
	}
}

func (p *StraceParser) ParseLine(line string) (e Event, complete bool, err error) {
	var cat, pid, ts, syscall, args, ret, duration string
	var m []string

	for _, r := range regexes {
		m = r.re.FindStringSubmatch(line)
		if len(m) != 0 {
			cat = r.category
			break
		}
	}

	if len(m) == 0 {
		return e, true, errors.New("unknown format")
	}
	if len(m) == 7 {
		pid, ts, syscall, args, ret, duration = m[1], m[2], m[3], m[4], m[5], m[6]
	}
	if len(m) == 5 {
		pid, ts, syscall, args = m[1], m[2], m[3], m[4]
	}
	if len(m) == 4 {
		pid, ts, args = m[1], m[2], m[3]
	}

	e.Name = syscall
	e.Cat = cat
	switch e.Cat {
	case "successful", "failed", "etc":
		e.Ph = "X" // Complete event
		complete = true
	case "detached":
		e.Ph = "X" // Complete event
	case "unfinished":
		e.Ph = "B" // Begin event
	}

	e.Args.FullLine = line
	e.Args.Syscall = syscall
	e.Args.SyscallArgs = strings.Trim(args, " ()")
	e.Args.Result = ret
	{
		f, err := parseDuration(duration)
		if err != nil {
			return e, complete, fmt.Errorf("cant parse duration %q: %w", duration, err)
		}
		e.Args.Duration = f
	}

	pidInt, err := strconv.ParseUint(pid, 10, 32)
	if err != nil {
		return e, complete, fmt.Errorf("cant parse pid %q: %w", pid, err)
	}
	e.PID = int(pidInt)
	e.TID = int(pidInt)

	t, err := parseTime(ts)
	if err != nil {
		return e, complete, fmt.Errorf("cant parse time: %w", err)
	}
	e.Timestamp = int(t.UnixNano())

	switch e.Cat {
	case "unfinished":
		p.prevUnfinished[e.PID] = e
	case "detached":
		prev := p.prevUnfinished[e.PID]
		if prev != (Event{}) {
			if prev.Args.Syscall != e.Args.Syscall {
				return e, complete, fmt.Errorf("failed to match continuation event with starting one: %q != %q", prev.Args.Syscall, e.Args.Syscall)
			}

			// merge "e" with "prev"
			e.Cat = "successful"
			e.Ph = "X"
			e.Timestamp = prev.Timestamp
			e.Args.SyscallArgs = prev.Args.SyscallArgs + e.Args.SyscallArgs
			complete = true
		}
	}

	return e, complete, nil
}

type TraceEvents struct {
	Event           []Event `json:"traceEvents"`
	DisplayTimeUnit string  `json:"displayTimeUnit"` // “ms” or “ns”
}

func parseTime(ts string) (time.Time, error) {
	dot := strings.IndexByte(ts, '.')
	if dot == -1 {
		return time.Time{}, fmt.Errorf("timestamp has wrong format")
	}

	sec, err1 := strconv.ParseUint(ts[:dot], 10, 64)
	usec, err2 := strconv.ParseUint(ts[dot+1:], 10, 64)
	if err1 != nil || err2 != nil {
		return time.Time{}, fmt.Errorf("timestamp has wrong format")
	}

	return time.Unix(int64(sec), int64(usec)*1000), nil
}

func parseDuration(duration string) (float64, error) {
	if duration == "" {
		return 0, nil
	}
	duration = strings.Trim(duration, " <>")
	return strconv.ParseFloat(duration, 64)
}
