// Package ecslua runs Lua 5.1 scripts (GopherLua, pure Go) as Weakpixel Engine systems.
//
// A script names a registered Game Object (group:class) and receives its
// components with field access by JSON name, validated against the GOD
// TypeSchema. See doc/guide for the full contract and
// examples.
//
// This is a separate module so the root Weakpixel module has no dependency
// on the Lua implementation.
package ecslua
