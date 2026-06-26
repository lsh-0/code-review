## 1. Dependency and schema package

- [x] 1.1 Add `cuelang.org/go` to `backend/go.mod` and run `go mod tidy`; confirm the build still produces a single binary with no `cue` CLI runtime dependency.
- [x] 1.2 Create the `backend/schema/` package directory.
- [x] 1.3 Write `backend/schema/statefile.cue`: declare `#SchemaVersion: "1.0.0"`; define a version-agnostic `#ReviewBody` covering the canonical `Review` shape — `id`, `repo_path`, `source_branch`, `target_branch`, optional opaque `_readme`, `files` (array of `{file_path, comments}`), file and review-level `comments` (with `id`, optional `parent_id`, `author`, `content`, `status` ∈ active|resolved|ignored, optional `anchors`), `anchors` (`blob`, `line_number`, optional `offset`, optional `context`), and `marked_files` (array of `{path, blob?}`) — left **open** (trailing `...`) so future ADDITIONs compose; then define `#Statefile: #ReviewBody & { version?: #SchemaVersion }` as the 1.0.0 schema. Do NOT fold the version literal into the shared shape.
- [x] 1.4 Write `backend/schema/README.md` recording the schema-evolution decision: CUE narrows and never overrides, so versions are composed from `#ReviewBody` rather than extended; optional→mandatory is a legal narrowing while changing a concrete value is a conflict; `#`-definitions are closed so the reusable shape stays open; and the SchemaVer mapping (ADDITION/REVISION/MODEL) with the `#Statefile_1_1_0: #ReviewBody & {...}` pattern for the next version. Source this from the "factor the structural shape from the version layer" decision in `design.md`.

## 2. Schema package API

- [x] 2.1 In `backend/schema/schema.go`, embed `statefile.cue` with `go:embed`, compile it once via `cuecontext` at package init, and cache the compiled `#Statefile` value and the extracted `#SchemaVersion`.
- [x] 2.2 Expose `Version` (read from `#SchemaVersion`, not a Go literal).
- [x] 2.3 Implement `Validate(review *model.Review) error` that encodes the in-memory model with `ctx.Encode`, unifies with `#Statefile`, runs `Validate(cue.Concrete(true))`, and returns an error naming the offending path on failure.
- [x] 2.4 Implement a pure `Classify(version string) Class` returning unversioned / current / mismatched by comparing against `Version`.

## 3. Model and storage integration

- [x] 3.1 Add `Version string \`json:"version,omitempty\"`` to `model.Review`.
- [x] 3.2 In `SaveReview`, stamp `review.Version = schema.Version` (alongside the existing `Readme` stamp), validate with `schema.Validate`, and return an error without writing if validation fails.
- [x] 3.3 In `LoadReview`, after `json.Unmarshal`, classify by the parsed `version`; for unversioned/current, run `schema.Validate` (distinguishing a schema error from a JSON parse error); for mismatched, return/report a needs-migration result without force-validating against the `1.0.0` schema.
- [x] 3.4 Decide and implement how the classification result is returned from `LoadReview` (e.g. an additional return value or a field), keeping it loggable now without requiring UI work.

## 4. Tests

- [x] 4.1 Add table-driven unit tests for `schema.Validate` over fixtures: a conforming current file, a conforming unversioned (normalized-legacy) review, and invalid reviews (missing `id`, wrong-typed field, bad `status`, non-`1.0.0` `version`).
- [x] 4.2 Add unit tests for `Classify` covering unversioned, current, and mismatched.
- [x] 4.3 Add save/load round-trip tests using `t.TempDir()`: saved file carries `version: "1.0.0"` and re-loads; a non-conforming review is not written; a legacy unversioned fixture loads and is classified pre-`1.0.0`.
- [x] 4.4 Run `go test ./...` in `backend/` and confirm all pass.
