// +game:group:name=example
//
// Package game declares the Game Objects of the Lua example: a Drifter that
// moves, bounces and slowly loses health. The Go types are generated into
// typed views (zz__gen.go) and a JSON definition (build/god/example.god.json
// in the Lua module); the Lua script in ../scripts sees the same objects through it.
package game

//go:generate go run github.com/weakpixel/engine/tools/wpix gen
