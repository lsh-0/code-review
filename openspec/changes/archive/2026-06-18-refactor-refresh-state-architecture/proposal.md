## Why

Selecting a file in the review re-runs `git diff` and re-parses the entire diff on every click,
because `RefreshState` fuses a cheap review-state reload with an expensive diff recompute and
`selectFile` calls it unconditionally. The diff does not change between selections, so this is a
synchronous subprocess paid for nothing — the visible pause when a file body first displays. The
same fusion has made the refresh path accrete hacks that the upcoming features (uncommitted-change
banners, an agent CLI that writes the state file, multi-select) would each extend, so the seam is
worth fixing before more is built on it.

## What Changes

- Split `RefreshState` into two backend operations: `ReloadReview` (re-read the state JSON, no git)
  and `RecomputeDiff` (run `git diff` and re-parse). File selection triggers neither; opening the
  overview triggers the cheap reload; the explicit Refresh button triggers both. **BREAKING** for the
  JS bridge: `RefreshState` is removed and replaced.
- Collapse the ~18 hand-wrapped frontend→backend bridge calls into one generic `callBackend` helper,
  so adding a backend method is one line rather than fifteen of undefined-guard and promise plumbing.
- Introduce a typed git-query layer (`gitquery.go`) separating the committed-branch `DiffQuery` from a
  new `GetWorkingTreeStatus` (uncommitted modified/deleted tracked files), and a pure
  `Review.ReconcileMarks` in `model` that drops marks for files changed since they were marked. This
  backs the uncommitted-changes banners and unmark-on-commit without re-touching the refresh path.
- Make a reviewer's own comment actions update incrementally: mutators return only the affected thread
  (`CommentMutationResult`) and the frontend patches that one DOM subtree instead of re-rendering the
  whole file or overview. This also stops a comment add from discarding expanded context lines and
  scroll position.
- Add a backend watcher on the state JSON. When a second writer (the agent/CLI) changes the file, the
  GUI shows a dismissable top-of-window banner — "changes to the review have been made [refresh]" — and
  reloads only when the reviewer clicks it. The view never changes underneath the reviewer.

## Capabilities

### New Capabilities

- `review-state-sync`: How the GUI loads, reloads, and recomputes review state and the diff — the
  separation of the cheap review-state axis from the expensive git axis, which action triggers which,
  and how a reviewer's own mutations update the view incrementally.
- `external-change-notification`: How the GUI detects that a second writer (agent or CLI) has changed
  the state file while it is open, and surfaces a reviewer-controlled refresh banner rather than
  mutating the view live.
- `working-tree-status`: The typed git-status query (modified/deleted tracked files against the working
  tree) distinct from the branch diff, and the pure reconciliation of marked files against newly
  changed files.

### Modified Capabilities

<!-- No existing spec's requirements change. The diff-context-expansion spec is unaffected: this
     refactor preserves expand-context behaviour and in fact stops a comment add from discarding it. -->

## Impact

- `backend/main.go`: `RefreshState` removed; `ReloadReview` and `RecomputeDiff` added; comment mutators
  return `CommentMutationResult`; watcher wired into `startup`.
- `backend/git.go` / new `backend/gitquery.go`: low-level git primitives stay; typed `DiffQuery` and
  `GetWorkingTreeStatus` added.
- new `backend/watcher.go`: file-watch on the state JSON, debounced, with self-write suppression,
  emitting a Wails event.
- `frontend/web.go`: `callBackend` helper replaces the per-method wrappers; `applyMutation` patches a
  single thread; `EventsOn` handler shows the refresh banner; `selectFile` stops calling refresh.
- `model/model.go`: pure `Review.ReconcileMarks` added.
- JS bridge surface changes (removed `RefreshState`, new `ReloadReview` / `RecomputeDiff` /
  `GetWorkingTreeStatus`, mutators return JSON). Wails v2.11 `EventsEmit`/`EventsOn` used for the first
  time, solely for the watcher→banner signal.
- Constraints unchanged: git, `//go:embed`, and file-watching stay backend-side (GopherJS cannot exec,
  embed, or watch files); `model` stays pure Go 1.19 with no new dependencies.
