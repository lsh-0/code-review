## Why

The frontend is written in Go and compiled to JavaScript via GopherJS. That choice promised seamless front-and-back sharing but delivered language-version pinning, a heavy build toolchain, a 37k-line compiled artefact that works against the project's "small and self-contained" goal, and — because the bridge is JSON-over-RPC in both directions — almost none of the type-sharing it was meant to provide. The single largest source of incidental complexity is `frontend/web.go`: 1,932 lines of imperative `*js.Object` DOM assembly that cannot be unit-tested and cannot use the JavaScript ecosystem (blocking, among other things, drop-in syntax highlighting). Replacing it with hand-written TypeScript on a Deno toolchain removes that complexity, restores a unit-testable pure core, and unblocks highlighting — without changing any backend behaviour.

## What Changes

- **BREAKING** (build only): remove GopherJS from the toolchain. `./manage.sh setup`/`build` no longer install or invoke GopherJS; the build instead type-checks, tests, and bundles TypeScript with Deno.
- Delete the `frontend/` Go module (`web.go`, `web_test.go`) and the compiled `assets/review.js`.
- Re-implement the frontend in TypeScript, split into three layers: a **pure core** (no DOM, no Wails) holding all diff/comment/expand-state logic; a **typed RPC client** wrapping the existing Wails `window.go.main.App.*` bindings; and a thin **render layer** that is the only code touching the DOM.
- The `model` Go package stops being dual-compiled — it becomes native-only, and every `//go:build js` / `//go:build !js` tag is removed from the codebase.
- Collapse the four-module `go.work` workspace to a single Go module (`go.work` removed), keeping `model` as a sibling package to `backend` rather than dissolving it.
- Add **unit tests** for the pure core (`deno test`, zero third-party dependencies) and **DOM integration tests** for the render layer (using the `@b-fuze/deno-dom` WASM backend, the sandbox-respecting option).
- Introduce **syntax highlighting** of diff code via highlight.js, treated as an opaque third-party effect at the render boundary (for rough visual differentiation) and curated to only the languages reviewed. The library's output is not tested; the project owns only language selection from the file path and graceful fallback to plain content.

No backend Go behaviour changes: the bound `App` methods, the state-file format, the git queries, and the watcher are all untouched. The diff still expands, marks still persist, state still syncs, banners still show — the requirements of the existing specs are preserved; only their rendering implementation is replaced.

## Capabilities

### New Capabilities
- `frontend-architecture`: the three-layer frontend structure (pure core / RPC client / render layer), the rule that non-trivial computation lives in the testable pure core and the render layer only reads state and calls DOM APIs, and the two test altitudes (dependency-free unit tests for the core, DOM-shim integration tests for rendering).
- `frontend-toolchain`: the Deno-based build — type-checking, testing, and bundling to the single JavaScript artefact the WebKit webview loads — with no `node_modules` and no GopherJS, and the constraint that the shipped binary embeds only the bundled JS (Deno is a build/test-time tool, never a runtime dependency of the app).
- `syntax-highlighting`: highlighting of diff code so source is differentiated at a glance, delegated to a third-party library at the render boundary (its output untested), with the project owning only language selection and plain-content fallback, curated to the reviewed language set to keep the artefact small.

### Modified Capabilities
<!-- None. The rewrite preserves the requirements of diff-context-expansion, durable-file-marks,
     external-change-notification, review-state-sync, and working-tree-status; it changes how they
     are rendered, not what they require. -->

## Impact

- **Removed**: `frontend/` Go module; `assets/review.js`; GopherJS toolchain dependency and its build step; the `callBackend` reflection bridge; duplicated type declarations across the JS/Go boundary; all `//go:build js`/`!js` tags; the `go.work`/`go.work.sum` workspace files (collapsed to one `go.mod`).
- **Added**: a Deno-managed TypeScript frontend (source, bundle step, tests); one curated third-party library (highlight.js) plus one test-only library (`@b-fuze/deno-dom`); a Deno-based build path in `manage.sh`.
- **Unchanged**: `backend/` Go (all `App` methods, git queries, storage, watcher); the state-file JSON schema and its `_readme`; `model` types and logic (now compiled once, natively); `assets/index.html` and `assets/style.css` (carried over, adjusted only as the new markup requires).
- **README/build docs**: installation and development instructions change to describe the Deno toolchain instead of GopherJS.
