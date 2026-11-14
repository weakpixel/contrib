// +game:group:name=test
//
// Package luatest declares the Game Objects of the ecslua tests. They are
// test fixtures with deliberately plain names (group "test", objects
// TestObject and TestExtended) and are not part of any game. The Lua module
// generates them itself (zz__gen.go here, the GOD under the ignored
// build/god), so its tests never depend on generated output of the root or
// examples modules.
package luatest

//go:generate go run github.com/weakpixel/engine/tools/weakpixel gen
