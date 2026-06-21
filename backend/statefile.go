package main

import _ "embed"

// statefileUsage is the instruction prose stamped into a state file's
// `_readme` field. It lives in `statefile-usage.md` so it can be edited as
// markdown rather than a Go string literal, and is embedded here in `backend`
// (rather than alongside the field's definition in `model`) so `SaveReview` can
// stamp it on write while keeping `model` free of an embed dependency.
//
//go:embed statefile-usage.md
var statefileUsage string
