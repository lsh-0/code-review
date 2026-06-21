## Context

The frontend is `frontend/web.go`: 1,932 lines compiled to JavaScript by GopherJS. It builds the entire UI imperatively through string-keyed `doc.Call("createElement", …)` calls, bridges to the backend through a reflection-based `callBackend` helper over `*js.Object`, and is gated behind `//go:build js` so it cannot be compiled or tested by the native Go test suite. The `model` package is dual-compiled (native + GopherJS) to share `Comment`/`Review` types across the boundary, but the boundary is JSON strings in both directions — every bound `App` method returns `(string, error)` and the frontend `json.Unmarshal`s it — so the shared types are re-declared on the JS side anyway and the type-sharing is largely notional.

The backend is sound and stays put. Wails already generates JS bindings at `backend/frontend/wailsjs/go/main/App.js`, so a non-GopherJS frontend has a ready-made way to call the Go `App` methods. The reviewer has chosen, in discussion: a Deno toolchain (built-in type-check/test/bundle, explicit-permission security, no `node_modules`); no UI framework (plain TS over the DOM); testing against a real DOM via a shim rather than asserting on markup strings; and syntax highlighting via a dependency-free library treated as an opaque effect at the render boundary, not as project logic to be tested (the reviewer's stated position: purity is wanted for our code, and we do not test another library's behaviour).

## Goals / Non-Goals

**Goals:**
- Remove GopherJS and its toolchain, build step, and 37k-line artefact entirely.
- Produce a frontend whose logic is unit-testable without a browser, restoring the testability the `model` package already enjoys to the rendering side that currently has none.
- Keep the shipped binary small and self-contained; Deno is a build/test-time tool only and never ships.
- Add syntax highlighting for rough visual differentiation, delegated to a third-party library at the render boundary (we test our language selection and fallback, not the library's output).
- Leave all backend Go behaviour, the state-file format, and the existing specs' requirements unchanged.

**Non-Goals:**
- No change to any bound `App` method signature, the state JSON, git queries, or the watcher.
- No UI framework, no virtual DOM, no reactive library.
- No new runtime capability in the shipped app (no Deno at runtime, no network, no new permissions).
- Not re-litigating the existing five specs' behaviour — this is a rendering re-implementation, not a behaviour change.
- Not pursuing performance gains; startup speed is already satisfactory.

## Decisions

**1. TypeScript over the DOM, no framework.** Rationale: matches the reviewer's preference to keep dependencies minimal and side effects at the margins, and avoids the npm-graph bloat a framework invites. Alternative considered: a tiny VDOM/template library (lit, van.js) to make render functions pure `state → markup`. Rejected for now to keep zero render-layer dependencies; the three-layer split below recovers most of the testability a template lib would offer without the dependency.

**2. Three-layer structure: pure core / RPC client / render layer.** The pure core holds every non-trivial computation already present in `web.go` but currently fused to DOM calls — the expand-gap state machine (`stepRange`, `gapExhausted`, `effectiveBoundary`, `advanceState`), `hunkReachedEOF`, comment selection/threading (`rootComments`, `getReplies`, `getCommentsForLine`, `threadAuthors`, `authorLabel`), context extraction (`getLineContext`), the overview visibility-window computation, and status derivation. The RPC client is one typed module wrapping the Wails bindings, replacing the `callBackend` reflection bridge with typed `async` functions. The render layer is the only code that touches the DOM and contains no non-trivial computation — it reads state, calls a pure function for any decision, and emits nodes. Rationale: this is the seam that makes the core unit-testable; it mirrors the backend's `model` (pure) vs `App` (effectful) split that already pays off in tests.

**3. Deno for type-check, test, and bundle.** `deno check`, `deno test`, `deno bundle` (reintroduced in Deno 2.4 on an esbuild core, stable since). One binary, no `node_modules`, no separate compiler or bundler. Rationale: collapses the toolchain to a single tool and fits a Go project's single-binary sensibility. Alternative considered: Node + `node:test` + jsdom + esbuild/tsc. Rejected — more moving parts and a `node_modules` tree, the exact bloat being escaped. Note: TypeScript 7.0 (`tsgo`) is, as of this writing, an RC, not GA; Deno's built-in TypeScript handling is the type-checker here, so the TS 7.0 release status does not gate this work.

**4. DOM integration tests via `@b-fuze/deno-dom`, WASM backend.** The pure core tests need no DOM and no dependency. The render layer is tested against a real DOM provided by deno-dom's WASM backend, which is self-contained (html5ever + nwsapi compiled into the WASM blob, no transitive package graph) and respects Deno's sandbox (needs only `--allow-read`/`--allow-net` for module init, unlike the native FFI backend). Rationale: lets rendering be verified by querying an actual tree rather than asserting on strings, per the reviewer's preference, with one contained test-only dependency. Limitation accepted: deno-dom is not the WebKitGTK engine, so it verifies that rendering produces the right tree, not that WebKit paints it correctly — renderer quirks remain a manual-verification concern (consistent with the known WebKitGTK quirks already documented for this project).

**5. Syntax highlighting via highlight.js, at the render boundary, curated languages.** Highlighting is treated as an opaque third-party effect layered over the rendered diff for rough visual differentiation — not as project logic. It is invoked from the render layer (the margin where side effects belong), and the library's output is NOT tested: testing highlight.js's tokenising would be testing someone else's parser. The project owns only two things: selecting the file's language from its path (a small, unit-tested `path → language` selector — that part *is* our logic), and ensuring a highlighting failure or an unrecognised language degrades to plain content without breaking the diff. The wiring mechanism — let the library walk the DOM nodes (`highlightElement`) versus hand it text and inject the result (`highlight(code, {language})` + `innerHTML`) — is deliberately left open and chosen at apply time against the line-anchored DOM. Note: per-line auto-detection is unreliable on single lines, so the language is supplied from the file extension regardless of mechanism. Register only the languages actually reviewed (e.g. Go, TS/JS, shell, JSON) rather than the full ~190-language build, to keep the bundled artefact small and honour the "<10MB, self-contained" goal. Rationale: highlight.js has no runtime dependencies; keeping it at the boundary lets our code stay pure around it without us owning its correctness; the size risk is handled by curation. Earlier framing that mandated a "pure transform, unit-tested" was a misapplication of the project's purity preference — purity is wanted for *our* logic, and the highlighter is not ours.

**6. `model` becomes native-only; build tags removed.** With GopherJS gone, `model` is compiled once by the Go toolchain. All `//go:build js` and `//go:build !js` tags are deleted. The TypeScript side re-declares the wire types it needs (it already did). Rationale: removes the dual-compilation constraint that forced, among other things, the `//go:embed`-in-`backend` workaround for the statefile readme.

**7. Collapse to one Go module, but keep `model` as a sibling package.** With the frontend no longer a Go module, the four-module `go.work` workspace loses its reason to exist (it scaffolded the dual-compilation boundary). Collapse to a single `go.mod` and drop `go.work` entirely. However, `model` SHALL remain a separate Go *package* within that module — it is not dissolved into `backend`. Rationale: `model`'s separateness today is partly a compiler artefact, but the package boundary itself is doing real architectural work — `model` is the pure, side-effect-free, independently-unit-tested kernel, and because it does not import `backend` the compiler enforces for free that domain logic cannot reach git, the filesystem, or `os/exec`. Folding it into the effectful `backend` package would downgrade that guarantee to discipline and mix the pure and git-touching test altitudes. Keeping it as a sibling package costs nothing once both are in one module, and it stays directly importable by the planned agent CLI (a second Go binary that will share the `Review`/`Comment` types) without dragging in the Wails/webview surface. Alternative considered: merge `model` into `backend`. Rejected — it is more change, not less, and it discards the one package boundary worth keeping. This is also the smaller diff: only the module/workspace wrapping changes; the package stays put.

## Risks / Trade-offs

- **Loss of compile-time type-sharing across the bridge** → The boundary is already JSON and types are already re-declared on the JS side, so the loss is near-notional. Mitigate by keeping the TS wire types in one module and asserting their shape against a fixture produced by the Go `App` methods, so drift is caught by a test rather than a compiler.
- **Render layer remains imperative and not pure** → No framework means nothing structurally prevents computation creeping back into DOM calls, the original `web.go` failure mode. Mitigate with the standing rule (render functions may read state and call DOM APIs but must not compute; any decision is a pure function called first) and by DOM integration tests that exercise the render path.
- **deno-dom fidelity gap** → Tests pass against a non-WebKit DOM. Mitigate by scoping DOM tests to structural assertions (right nodes, classes, attributes, ordering) and leaving rendering/paint correctness to manual checks in the real webview, as is already the practice for WebKitGTK quirks.
- **highlight.js bundle size** → The full build is large. Mitigate by importing core + a curated language list only; treat the bundled artefact size as a checked output, not an assumption.
- **`deno bundle` + WASM interaction** → deno-dom's WASM module historically needed care under `deno bundle` (the `wasm-noinit`/`initParser()` no-op). This affects only bundling deno-dom; it is a test-only dependency and is never bundled into the shipped artefact, so it does not reach the build output. Note it so it is not rediscovered the hard way.
- **This is a rewrite, not a refactor** → ~1,900 lines of DOM logic are re-expressed in TS. Mitigate by porting layer-by-layer behind the existing bound `App` methods (unchanged), and by carrying `index.html`/`style.css` over largely intact so only the JS is new.

## Migration Plan

1. Stand up the Deno toolchain in `manage.sh` alongside the existing GopherJS path (do not remove GopherJS yet); confirm `deno check`/`test`/`bundle` run.
2. Port the pure core to TS first, with unit tests, validated independently of any DOM or Wails.
3. Add the typed RPC client over the generated Wails bindings; pin wire-type shapes against fixtures from the Go `App` methods.
4. Port the render layer, with DOM integration tests; carry over `index.html`/`style.css`.
5. Add the `path → language` selector (with tests) and wire highlight.js in at the render boundary, with a plain-content fallback; do not test the library's highlighted output.
6. Switch the build to emit the Deno-bundled JS into `assets/`; verify the app runs in the real WebKit webview against the existing backend.
7. Remove GopherJS: delete `frontend/`'s Go files and `assets/review.js` (GopherJS output), strip the GopherJS setup/build steps, drop the build tags, make `model` native-only, trim `go.work`, update the README.

Rollback: until step 7, the GopherJS frontend remains buildable; reverting is dropping the Deno path and rebuilding the old artefact. After step 7, rollback is a git revert of the change.

## Open Questions

- How does Wails expect the bundled JS to be delivered — a single embedded `assets/review.js` (current shape) or ES modules — and does that imply bundling vs per-file transpilation? Resolve before finalising the bundle step.
- Final curated highlight.js language set: confirm which languages the reviewer actually diffs often enough to warrant inclusion.
- Whether `index.html`/`style.css` need structural changes for the new markup, or carry over unchanged.
