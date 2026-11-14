package main

import (
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	var out strings.Builder
	if err := run([]string{"-ticks", "6"}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"alpha", "beta", "gamma", `final state: {"objects":[`, `"group":"example","class":"Drifter"`} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	// beta starts with 3 hit points, so it was replaced (despawn + spawn from Lua).
	if !strings.Contains(got, "beta'") {
		t.Errorf("no replacement of beta in:\n%s", got)
	}
}

func TestRunBadFlag(t *testing.T) {
	var out strings.Builder
	if err := run([]string{"-nope"}, &out); err == nil {
		t.Fatal("want an error for an unknown flag")
	}
}
