## Context

The app is a Wails v2.11 desktop GUI across four Go modules joined by `go.work`: `backend` (native,
`//go:build !js`), `frontend` (GopherJS-compiled to JS, `//go:build js`, capped at Go 1.19),
`model` (shared, compiled by both — so Go 1.19 and no `//go:embed`), and `assets` (embedded HTML/CSS/JS,
backend-only). The frontend reaches the backend over a JS bridge: every call is
`win.Get("go").Get("main").Get("App").Call("Method", args...)` returning a promise.

`RefreshState` (`backend/main.go`) reloads the state JSON and then calls `loadDiff()`, which shells out
to `git diff base...head`, re-parses, and rebuilds the file-content cache. `selectFile`
(`frontend/web.go`) calls `refreshState` → `RefreshState` on every file click, so each selection pays a
synchronous git subprocess for a diff that has not changed. The frontend keeps parallel state
(`diffFiles`, `commentsCache`, `markedFiles`) reconciled by brute re-fetch and full re-render after every
mutation (`refreshAfterAction`), which discards expanded context and scroll. Upcoming features
(uncommitted-change banners, an agent CLI that becomes a second writer to the state file, multi-select)
would each extend this fused path.

GopherJS cannot `exec`, `//go:embed`, or watch files, so git, embedding, and file-watching must stay
backend-side. Wails v2.11 ships `EventsEmit`/`EventsOn`, currently unused.

## Goals / Non-Goals

**Goals:**

- File selection performs no git work; the per-click `git diff` is removed from the hot path.
- `RefreshState`'s two responsibilities are split into `ReloadReview` (cheap JSON) and `RecomputeDiff`
  (git), with a clear trigger matrix for each user action.
- One generic `callBackend` helper replaces the ~18 hand-wrapped bridge calls.
- A typed git-query layer separates the branch diff from working-tree status, backing the
  uncommitted-change banners and a pure marked-file reconciliation.
- Comment mutations patch a single thread, preserving expanded context and scroll.
- External edits to the state file surface a reviewer-controlled refresh banner; the view never changes
  underneath the reviewer.
- Each stage is independently shippable and testable, in keeping with the project's preference for pure
  functions and simple unit tests.

**Non-Goals:**

- No live incremental re-render of externally-made changes. External edits only raise a banner; the
  reviewer chooses when to absorb them.
- No multi-select implementation here. The state-ownership changes make it cheaper later, but it is its
  own change.
- No syntax highlighting, GitHub/Bitbucket import, or CLI implementation here. The git-query layer and
  second-writer handling prepare for them; they ship separately.
- No change to the state-file schema or to `diff-context-expansion` behaviour. Expand-context is
  preserved (and stops being discarded on comment add).

## Decisions

### Two axes, two operations

Split `RefreshState` into `ReloadReview() error` (re-read JSON into `a.review`, no git) and
`RecomputeDiff() error` (the current `loadDiff` body: `git diff`, `ParseDiff`, reset file cache,
reconcile `AddFileDiff`). Delete `RefreshState`. The trigger matrix:

| Action | Git axis | Review axis |
|---|---|---|
| Select file | none | none |
| Open overview | none | `ReloadReview` |
| Comment mutation | none | in-memory mutate + save |
| Refresh button | `RecomputeDiff` | `ReloadReview` |
| External-change banner refresh | none (default) | `ReloadReview` |

*Alternative considered:* a single `RefreshState(recomputeDiff bool)` flag. Rejected — a boolean
parameter to pick between two behaviours is the fusion restated, and it reads worse at call sites than
two named methods.

The correctness point: today `selectFile` refreshes partly so an agent's just-added comments appear on
click. Under the split, that need is served by the external-change banner (below) and the explicit
Refresh, not by a git diff on every click. Select becomes a pure view operation.

### `callBackend` helper

Add `app()` (resolve the bound App once, return nil if the bridge isn't ready) and
`callBackend(method string, out interface{}, onOK func(), args ...interface{})` which calls the method,
JSON-unmarshals the string result into `out` when non-nil, invokes `onOK`, and routes errors through a
single `reportBridgeError`. Every wrapper collapses to a few lines; adding a backend method is one
`callBackend` call. *Alternative considered:* code-generating typed wrappers. Rejected — heavier
machinery for a frontend that is already hand-written Go; the generic helper is enough.

### Typed git-query layer

Keep `git.go` for low-level `exec` primitives (`GetGitRoot`, `GetDiff`, `GetFileAtRevision`). Add
`backend/gitquery.go` with a typed `DiffQuery{RepoPath, Base, Head}` wrapping `GetDiff` + `ParseDiff`,
and a new `GetWorkingTreeStatus(repoPath) (WorkingTreeStatus, error)` over `git status --porcelain`
returning modified and deleted tracked paths (untracked excluded). They are distinct because they answer
different questions and feed different UI; conflating them is what made `RefreshState` a mess. The
marked-file reconciliation is a pure `Review.ReconcileMarks(changedPaths []string)` in `model`,
unit-tested without git, called from `RecomputeDiff`. *Alternative considered:* deriving working-tree
status from a second `git diff`. Rejected — `status --porcelain` is the precise, stable query for this.

### Incremental mutation result

Comment mutators (`AddComment`, `AddReply`, `UpdateComment`, `ResolveComment`, `IgnoreComment`,
`ReactivateComment`, `DeleteComment`) change from returning `error` to returning a JSON
`CommentMutationResult { FilePath, LineNumber, RootID, Comments, FileStatus }` describing the one thread
that changed and the file's recomputed status. This is a backend wire type, not a `model` type, so it
does not burden the GopherJS-compiled `model`. The frontend gains `applyMutation(result)`: overwrite
`commentsCache[FilePath]`, locate the thread's DOM node by a stable `data-line`/`data-root` attribute,
and re-render just that subtree (or remove it on delete); update the file pill from `FileStatus`.
`refreshAfterAction`'s full re-render is retired. Because the diff is never re-rendered on a mutation,
expanded context and scroll survive — this is also the fix for the preserve-page-state TODO.

### Watcher → banner, not live takeover

Add `backend/watcher.go` watching `a.statePath`, debounced (~200ms; the `bep/debounce` dep is already
present), with self-write suppression so the GUI's own `SaveReview` does not echo back as an external
change (guard a flag/generation around `SaveReview`, or compare a content hash). On a genuine external
change: `ReloadReview()` then `runtime.EventsEmit(a.ctx, "review:changed")`. The frontend registers
`EventsOn("review:changed", ...)` once in `initialize()` and shows a dismissable top-of-window banner;
it reloads only when the reviewer clicks refresh. *Alternatives considered:* (a) live incremental
re-render on external change — rejected, the reviewer explicitly does not want the view changing
underneath them, and a spurious self-write echo rewriting the DOM is a real hazard whereas a spurious
banner is harmless; (b) manual-refresh-only — rejected, the core async round-trip means the reviewer
walks away and returns to a silently stale GUI; (c) reload-on-focus — rejected as coarser than watching
the actual file and still risks changing the view without consent.

### Sequencing

Stage 1 (split `RefreshState`) ships alone and is the immediate perf win with no bridge-shape change.
Stage 2 (`callBackend`) is a mechanical refactor that makes 3–5 shorter. Stage 3 (git-query layer +
working-tree status + `ReconcileMarks`) adds the typed queries and unmark-on-commit. Stage 4
(`CommentMutationResult` + `applyMutation`) delivers incremental updates and the page-state fix. Stage 5
(watcher + banner) builds on the earlier stages and ships last as the most stateful piece.

## Risks / Trade-offs

- **Self-write echo** → without suppression, every `SaveReview` fires the watcher and shows a spurious
  banner. Mitigation: suppress the next watcher event around each GUI write (generation counter or
  content-hash compare); cover with a backend test that a GUI save raises no notification.
- **Stale view after split** → removing refresh-on-select means an agent's edits no longer appear on
  click. Mitigation: this is by design — the banner (Stage 5) and explicit Refresh are the absorption
  points; until Stage 5 ships, Refresh remains the path, unchanged from today's behaviour for the
  reviewer.
- **Debounce window dropping a rapid second external edit** → a write landing just after the reload but
  within the same coalesce window could be missed. Mitigation: re-arm the watcher after each reload;
  worst case the next write re-raises the banner.
- **`data-line`/`data-root` addressing drift** → if thread DOM attributes are absent or wrong,
  `applyMutation` patches the wrong node. Mitigation: set the attributes in the single
  `createCommentThread` path and assert them in a frontend test before relying on lookup.
- **Bridge-surface break** → removing `RefreshState` and changing mutator return types is breaking for
  any caller. Mitigation: the only caller is this frontend, changed in lockstep; no external consumer.

## Migration Plan

Each stage is a separate commit that builds, passes tests, and runs. Order: 1 → 2 → 3 → 4 → 5. Stage 1
is the standalone perf fix and can be released on its own. Rollback is per-stage revert; no data
migration is needed because the state-file schema is unchanged. Verification is per stage: backend tests
assert `ReloadReview` invokes no git and `RecomputeDiff` does (the file cache already takes an injected
fetcher, giving the seam); `model` tests cover `ReconcileMarks`; frontend tests cover `applyMutation`
patching one thread; a manual pass confirms file-select has no perceptible pause and that an external
edit raises the banner without changing the view.

## Open Questions

- Self-write suppression mechanism: generation counter versus content-hash compare. Both work; pick
  during Stage 5 against whichever is simpler to test.
- Whether `fsnotify` (transitively present via Wails) is used directly or a backend mtime poll suffices.
  Decide at Stage 5; both stay backend-side and invisible to GopherJS.
