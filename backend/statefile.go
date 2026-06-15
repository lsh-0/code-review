//go:build !js

package main

import _ "embed"

// statefileUsage is the instruction prose stamped into a state file's
// `_readme` field. It lives in `statefile-usage.md` so it can be edited as
// markdown rather than a Go string literal; it is embedded here because the
// `model` package that defines the field is also compiled by GopherJS, which
// does not support `//go:embed`.
//
//go:embed statefile-usage.md
var statefileUsage string
