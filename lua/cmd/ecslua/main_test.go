package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var sample = filepath.Join("..", "..", "internal", "stubs", "testdata", "sample.god.json")

func TestStubs(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "sub", "god.lua")
		var stderr bytes.Buffer
		if code := run([]string{"stubs", "-o", out, sample}, nil, &stderr); code != 0 {
			t.Fatalf("code %d: %s", code, &stderr)
		}
		data, err := os.ReadFile(out)
		if err != nil || !strings.HasPrefix(string(data), "---@meta\n") {
			t.Fatalf("output: %v, %.40s", err, data)
		}
	})
	t.Run("stdout with title", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"stubs", "-o", "-", "-title", "My Game", sample}, &stdout, &stderr)
		if code != 0 || !strings.Contains(stdout.String(), "-- My Game: LuaLS definitions") {
			t.Fatalf("code %d: %s\n%s", code, &stderr, &stdout)
		}
	})
	t.Run("missing input", func(t *testing.T) {
		var stderr bytes.Buffer
		if code := run([]string{"stubs", "-o", "-", "nope.god.json"}, nil, &stderr); code != 1 || !strings.Contains(stderr.String(), "nope.god.json") {
			t.Fatalf("code %d: %s", code, &stderr)
		}
	})
	t.Run("usage", func(t *testing.T) {
		var stderr bytes.Buffer
		if code := run(nil, nil, &stderr); code != 2 {
			t.Fatalf("code %d", code)
		}
	})
}
