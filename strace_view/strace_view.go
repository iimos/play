package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed template.html
var templateHTML string

//go:embed script.js
var scriptJS string

//go:embed style.css
var styleCSS string

//go:embed syscalls.json
var syscallsJSON string

func main() {
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
	out.WriteString(html)

	fmt.Printf("result written in %q\n", outpath)
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
