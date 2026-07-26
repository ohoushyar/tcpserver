package main

import (
	"errors"
	"flag"
	"slices"
	"testing"
	"time"
)

func TestParseCmd(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCmd  string
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "simple command",
			input:    "tr",
			wantCmd:  "tr",
			wantArgs: []string{},
		},
		{
			name:     "multi-arg",
			input:    "tr a-z A-Z",
			wantCmd:  "tr",
			wantArgs: []string{"a-z", "A-Z"},
		},
		{
			name:     "skips empty args from extra spaces",
			input:    "echo  hello",
			wantCmd:  "echo",
			wantArgs: []string{"hello"},
		},
		{
			name:     "trims leading and trailing spaces",
			input:    "  tr a-z A-Z  ",
			wantCmd:  "tr",
			wantArgs: []string{"a-z", "A-Z"},
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   " ",
			wantErr: true,
		},
		{
			name:    "tabs and spaces only",
			input:   " \t  ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, args, err := parseCmd(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCmd(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if cmd != tt.wantCmd {
				t.Errorf("cmd = %q, want %q", cmd, tt.wantCmd)
			}
			if !slices.Equal(args, tt.wantArgs) {
				t.Errorf("args = %#v, want %#v", args, tt.wantArgs)
			}
		})
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    config
		wantErr bool
		errIs   error
	}{
		{
			name: "defaults with cmd",
			args: []string{"--cmd", "tr a-z A-Z"},
			want: config{
				addr:    "127.0.0.1:3131",
				cmd:     "tr",
				cmdArgs: []string{"a-z", "A-Z"},
			},
		},
		{
			name: "all flags",
			args: []string{
				"-v",
				"--addr", "0.0.0.0:4000",
				"--cmd", "tee",
				"--io-timeout", "5s",
			},
			want: config{
				addr:      "0.0.0.0:4000",
				cmd:       "tee",
				cmdArgs:   []string{},
				ioTimeout: 5 * time.Second,
				verbose:   true,
			},
		},
		{
			name:    "missing cmd",
			args:    []string{"--addr", "127.0.0.1:3131"},
			wantErr: true,
		},
		{
			name:    "help",
			args:    []string{"-h"},
			wantErr: true,
			errIs:   flag.ErrHelp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFlags(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFlags(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if tt.errIs != nil && !errors.Is(err, tt.errIs) {
				t.Fatalf("error = %v, want %v", err, tt.errIs)
			}
			if tt.wantErr {
				return
			}
			if got.addr != tt.want.addr {
				t.Errorf("addr = %q, want %q", got.addr, tt.want.addr)
			}
			if got.cmd != tt.want.cmd {
				t.Errorf("cmd = %q, want %q", got.cmd, tt.want.cmd)
			}
			if !slices.Equal(got.cmdArgs, tt.want.cmdArgs) {
				t.Errorf("cmdArgs = %#v, want %#v", got.cmdArgs, tt.want.cmdArgs)
			}
			if got.ioTimeout != tt.want.ioTimeout {
				t.Errorf("ioTimeout = %v, want %v", got.ioTimeout, tt.want.ioTimeout)
			}
			if got.verbose != tt.want.verbose {
				t.Errorf("verbose = %v, want %v", got.verbose, tt.want.verbose)
			}
		})
	}
}
