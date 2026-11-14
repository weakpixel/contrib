package ecslua_test

import (
	"embed"
	"testing"
	"testing/fstest"
	"time"

	ecslua "github.com/weakpixel/contrib/lua"
)

//go:embed testdata/embedded/set.lua
var embedded embed.FS

func TestSystemFSMapFS(t *testing.T) {
	f := newFixture(t, ecslua.Options{WatchInterval: tick})
	e := f.objs.NewEntity(obj(1))
	fsys := fstest.MapFS{"scripts/set.lua": {Data: []byte(healthSetter("10")), ModTime: time.Unix(100, 0)}}
	f.sched.AddRunner(f.rt.NewSystemFS(fsys, "scripts/set.lua"))
	f.sched.Run(f.world, tick)
	if got := hp(f, e); got != 10 {
		t.Fatalf("hp = %d, want 10", got)
	}

	fsys["scripts/set.lua"] = &fstest.MapFile{Data: []byte(healthSetter("20")), ModTime: time.Unix(200, 0)}
	f.sched.Run(f.world, tick)
	f.noErr()
	if got := hp(f, e); got != 20 {
		t.Fatalf("hp = %d after edit, want 20", got)
	}

	fsys["scripts/set.lua"] = &fstest.MapFile{Data: []byte("\n\nnot lua"), ModTime: time.Unix(300, 0)}
	f.sched.Run(f.world, tick)
	f.wantErr("scripts/set.lua line:3")
}

func TestSystemFSEmbed(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	e := f.objs.NewEntity(obj(1))
	s := f.rt.NewSystemFS(embedded, "testdata/embedded/set.lua")
	f.sched.AddRunner(s)
	f.sched.Run(f.world, tick)
	if got := hp(f, e); got != 77 {
		t.Fatalf("hp = %d, want 77", got)
	}
	if err := s.Reload(); err != nil { // same bytes: a harmless no-op
		t.Fatal(err)
	}
}

func TestSystemFSMissingFile(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	f.sched.AddRunner(f.rt.NewSystemFS(fstest.MapFS{}, "nope.lua"))
	f.sched.Run(f.world, tick)
	f.wantErr("nope.lua")
}

func TestNewSystemFSPanics(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	tests := []struct {
		name string
		call func()
		want string
	}{
		{"nil fs", func() { f.rt.NewSystemFS(nil, "a.lua") }, "lua: nil fs"},
		{"rooted", func() { f.rt.NewSystemFS(fstest.MapFS{}, "/a.lua") }, "lua: invalid script name /a.lua"},
		{"dotdot", func() { f.rt.NewSystemFS(fstest.MapFS{}, "../a.lua") }, "lua: invalid script name ../a.lua"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if msg, _ := recover().(string); msg != tc.want {
					t.Fatalf("panic %q, want %q", msg, tc.want)
				}
			}()
			tc.call()
		})
	}
}
