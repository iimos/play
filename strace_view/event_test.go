package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStraceLine(t *testing.T) {
	tests := []struct {
		in     string
		expect Event
	}{
		{
			in: `2270  1669729913.698444 execve("./bin", ["./bin"], 0x7ffeb48e4f18 /* 13 vars */) = 0`,
			expect: Event{
				Name:      "execve",
				Cat:       "successful",
				Ph:        "X",
				PID:       2270,
				TID:       2270,
				Timestamp: 1669729913698444000,
				Args: Args{
					Syscall:     "execve",
					SyscallArgs: `"./bin", ["./bin"], 0x7ffeb48e4f18 /* 13 vars */`,
					Result:      "0",
				},
			},
		},
		{
			in: `2270  1669729913.698444 execve("./bin", ["./bin"], 0x7ffeb48e4f18 /* 13 vars */) = 0 <0.011819>`,
			expect: Event{
				Name:      "execve",
				Cat:       "successful",
				Ph:        "X",
				PID:       2270,
				TID:       2270,
				Timestamp: 1669729913698444000,
				Args: Args{
					Syscall:     "execve",
					SyscallArgs: `"./bin", ["./bin"], 0x7ffeb48e4f18 /* 13 vars */`,
					Result:      "0",
					Duration:    0.011819,
				},
			},
		},
		{
			in: `2273  1669729914.298607 <... sigaltstack resumed>NULL)     = 0`,
			expect: Event{
				Name:      "sigaltstack",
				Cat:       "detached", // maybe it's wrong
				Ph:        "X",        // maybe it's wrong
				PID:       2273,
				TID:       2273,
				Timestamp: 1669729914298607000,
				Args: Args{
					Syscall:     "sigaltstack",
					SyscallArgs: `NULL`,
					Result:      "0",
				},
			},
		},
	}
	for _, tt := range tests {
		p := NewStraceParser()
		got, _, err := p.ParseLine(tt.in)
		if assert.NoError(t, err) {
			assert.Equal(t, tt.expect, got, tt.in)
		}
	}
}
