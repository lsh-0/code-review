## 1. Stand up the Deno toolchain (GopherJS still intact)

- [x] 1.1 Add a Deno project config (`deno.json`) with tasks for `check`, `test`, and `bundle`, and an import map pinning `@b-fuze/deno-dom` and highlight.js
- [x] 1.2 Resolve the open question of how Wails expects the bundled JS delivered (single embedded file vs ES modules); record the decision in `deno.json`'s bundle target
- [x] 1.3 Add Deno-based `check`/`test`/`bundle` steps to `manage.sh` alongside the existing GopherJS build (do not remove GopherJS yet)
- [x] 1.4 Verify `deno check`, `deno test`, and `deno bundle` all run on a trivial placeholder module

## 2. Port the pure core (no DOM, no bridge)

- [x] 2.1 Declare the TypeScript wire types (Comment, FileDiff, DiffHunk, DiffLine, mutation result) in one module
- [x] 2.2 Port the context-expansion state machine: `stepRange`, `gapExhausted`, `effectiveBoundary`, `advanceState`, and `hunkReachedEOF`
- [x] 2.3 Port comment selection/threading/authorship: `rootComments`, `getReplies`, `getCommentsForLine`, `threadAuthors`, `authorLabel`, and file-status derivation
- [x] 2.4 Port context extraction (`getLineContext`) and the overview visibility-window computation
- [x] 2.5 Write dependency-free unit tests for 2.2–2.4 using `given`/`expected`/`actual`, asserting requirements not structure
- [x] 2.6 Confirm the core compiles and tests pass with no DOM shim present

## 3. Typed RPC client over the Wails bindings

- [x] 3.1 Implement a typed client module wrapping `window.go.main.App.*`, one typed async method per bound `App` method, with a not-ready guard in one place
- [x] 3.2 Decode each method's JSON result into the wire types from 2.1 (no per-call-site reflection)
- [x] 3.3 Pin the wire-type shapes against fixtures captured from the Go `App` methods, so cross-bridge drift fails a test
- [x] 3.4 Type-check the client with `deno check`

## 4. Render layer and DOM integration tests

- [x] 4.1 Carry over `assets/index.html` and `assets/style.css`, adjusting only as the new markup requires (no change needed: the new TS targets the same element IDs and emits the same class names; only `<script src="review.js">` stays, now fed by the Deno bundle)
- [x] 4.2 Port file-list rendering, mark checkbox, and status pills, calling pure functions for every decision
- [x] 4.3 Port diff rendering: hunks, diff lines, line-number comment affordance, and embedded comment threads
- [x] 4.4 Port the expand affordances and the between-hunk merge, driven by the pure state machine from 2.2
- [x] 4.5 Port the overview surface (review-level section, per-file commented hunks, browse links)
- [x] 4.6 Port the comment modals, mutation application, and incremental thread/pill patching (preserving scroll and expanded context)
- [x] 4.7 Port the external-change banner, refresh flow, zoom, and wheel/keyboard handlers
- [x] 4.8 Write DOM integration tests (against `@b-fuze/deno-dom` WASM backend, `--allow-read`/`--allow-net`) asserting the produced tree's nodes, classes, attributes, and ordering

## 5. Syntax highlighting

- [x] 5.1 Add highlight.js with only the curated reviewed languages registered (confirm the set with the reviewer); verify the bundle omits the full language set
- [x] 5.2 Implement a `path -> hljs-language` selector keyed on file extension, with a plain-text fallback for unrecognised languages; unit-test the selector (this is our logic, not the library's)
- [x] 5.3 Wire highlight.js into diff-line rendering at the render boundary (mechanism: hand the library text and inject the escaped/highlighted result as `innerHTML` on `.line-content`; a highlighting failure or unrecognised language falls back to escaped plain content)

## 6. Switch the build over and verify in the real webview

- [x] 6.1 Make the Deno bundle the asset embedded and served by the backend (replacing the GopherJS `review.js` path) — verified: `deno task bundle` writes `assets/review.js`, which `assets/assets.go` `//go:embed`s and `backend/main.go` serves via `assets.Assets`; confirmed the artefact is the Deno/esbuild bundle with zero GopherJS markers
- [x] 6.2 Run the app against the unchanged backend in the real WebKitGTK webview; verify diff view, comments, marks, overview, expansion, banners, and highlighting all work — confirmed working end to end against a real Go diff (required two fixes found in the webview: the bridge-readiness wait, and the bundle `--format iife` wrap that stopped a minified global `go` clobbering Wails' `window.go` bindings)
- [x] 6.3 Manually check the known WebKitGTK-quirk areas (scrollbar, Ctrl-z/Ctrl-f, placeholder) are no worse than before — no regressions observed

## 7. Remove GopherJS and clean up

- [x] 7.1 Delete `frontend/web.go` and `frontend/web_test.go` (the whole `frontend/` Go module); `assets/review.js` is now the committed Deno bundle, no longer a GopherJS output
- [x] 7.2 Remove the GopherJS setup/build steps and the GopherJS Go-version pin from `manage.sh` (build/release now run `deno task bundle`; `setup` checks for Deno; gopher.* commands removed)
- [x] 7.3 Make `model` native-only: removed every `//go:build !js` tag from the backend files; updated the statefile `//go:embed` comment now that `model` is no longer dual-compiled
- [x] 7.4 Removed `go.work`/`go.work.sum` and collapsed to a single Go module `code-review` rooted at `backend/`, with `model` and `assets` as packages within it (no per-module `go.mod`, no `replace` directives). `model` stays a sibling package importing nothing effectful (only `crypto`/`encoding`); `backend` imports `model`/`assets`, not vice versa. (Initially landed as three modules wired by `replace` directives, then consolidated to the single module in a follow-up commit.)
- [x] 7.5 Update `README.md` installation/development instructions to describe the Deno toolchain instead of GopherJS
- [x] 7.6 Run the full Go test suite and the Deno test suite; confirm both pass and the binary still builds and runs — model + backend Go tests pass, 62 Deno tests pass, `go build` succeeds, and the installed binary runs correctly in the webview
