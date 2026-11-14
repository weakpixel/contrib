package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	sample := filepath.Join("..", "..", "internal", "stubs", "testdata", "sample.god.json")
	out := filepath.Join(t.TempDir(), "sub", "god.lua")
	tests := []struct {
		name     string
		args     []string
		code     int
		inStdout string
		inStderr string
	}{
		{"stdout", []string{"stubs", "-title", "My Game", sample}, 0, "-- My Game: LuaLS definitions", ""},
		{"file", []string{"stubs", "-o", out, sample}, 0, "", ""},
		{"no command", nil, 2, "", "usage: ecslua stubs"},
		{"unknown command", []string{"doc"}, 2, "", "usage: ecslua stubs"},
		{"no inputs", []string{"stubs"}, 2, "", "usage: ecslua stubs"},
		{"bad input", []string{"stubs", "missing.god.json"}, 1, "", "ecslua stubs: stat missing.god.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != tt.code {
				t.Fatalf("code %d, want %d: %s", code, tt.code, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.inStdout) || !strings.Contains(stderr.String(), tt.inStderr) {
				t.Errorf("stdout %.80q, stderr %q", stdout.String(), stderr.String())
			}
		})
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.HasPrefix(string(data), "---@meta\n") {
		t.Errorf("output file: %v %.40q", err, data)
	}
}
