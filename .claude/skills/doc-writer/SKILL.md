---
name: doc-writer
description: Use whenever documentation has to be written or updated, including pages in doc/, README.md, Go doc comments, plan-file doc changes, and doc fixes after a code change.
model: haiku
---

# Documentation writer

Write documentation for a contribution of this repository (a folder such as `lua/`, each its own Go module). This skill may run on a small model, so keep to the rules below and verify claims against the code instead of guessing.

## Before writing

1. Read the page you will change, in the contribution's `doc/` (for Lua: `lua/doc/guide/` and `lua/doc/design.md`).
2. Decide the home: the contribution's `doc/guide/` for users (public API and behavior), its `doc/design.md` for how and why it is built, its `README.md` for the overview and quick start, the repository `README.md` only for the list of contributions. Every topic has one home; link to it instead of repeating it.
3. Read the code the text describes (exported identifiers, panic messages, defaults). Never document behavior you have not seen in the code or the spec.

## Rules

- **Code and doc must agree.** If they differ, don't silently pick one: tell the caller. Fix the doc if the doc is wrong; report a code bug instead of documenting it as intended.
- **Behavior the docs left open** (for example the exact `: <detail>` suffix of a panic): document what the code does, in the same change.
- **Error and panic messages are API.** Copy them exactly into the guide (for Lua the `lua: ` prefix).
- **Guide pages** state the public API and behavior: what it does, a minimal runnable example, contracts (ownership, pointer validity, cost), panics. No internals.
- **Engine add-ons.** Contributions build on the Weakpixel Engine and may name it. Link engine docs with absolute GitHub URLs (`https://github.com/weakpixel/engine/blob/main/doc/...`), never with relative paths out of the repository.
- **Design page** explains why. Link to the guide for signatures; do not copy the API.
- **Go doc comments**: every exported identifier gets one stating what it does, its contract (preconditions, ownership, pointer validity, cost) and which panics it can raise. Start with the identifier's name. Explain non-obvious performance tricks inline.
- **New pages** are linked from the contribution's guide index (`doc/guide/README.md`).

## Style

- Plain, short sentences. Present tense, active voice. Say "returns", not "will return".
- Relative markdown links between pages of this repository. File names are kebab-case.
- Code blocks have a language tag. Examples must compile against the current API.
- Use tables for enumerations (options, panics), prose for reasoning.
- No marketing wording, no filler, no emojis, no em dashes.

## After writing

1. Check every link and identifier you added exists (`grep` the symbol, open the linked file).
2. If code samples changed, make sure they match the current API.
