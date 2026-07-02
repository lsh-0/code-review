## Why

The review diffs the committed branch against main, so a tracked file changed or
deleted on disk after the last commit is invisible to the reviewer: they may
approve a file whose working-tree copy no longer matches what they are reading.
The `working-tree-status` capability already answers "what has changed in the
working tree" as a typed query (`GetWorkingTreeStatus`), and the query is already
bound and exposed to the frontend — but nothing in the UI consumes it. The
reviewer gets no signal that the diff may be stale.

This change surfaces that signal with two banners and a link to the reviewer's
own diff tool, so a dirty working tree is visible rather than silent.

## What Changes

- A page-wide banner SHALL appear when the working-tree status reports any tracked
  file modified or deleted, counting modified and deleted files. It hides on a
  clean tree and refreshes with every review refresh (a new commit can change what
  is uncommitted). Untracked files are excluded by the existing query, so new
  files are ignored as before.
- Both banners SHALL update on their own shortly after a change is made on disk,
  without the reviewer having to trigger a manual refresh. A backend poller
  periodically re-runs the working-tree query and, when the status changes, emits
  an event the frontend reacts to by re-fetching and re-rendering the banners. This
  reuses the periodic-git-query approach already in place for the review-state
  watcher rather than monitoring individual files.
- A per-file warning banner SHALL appear when the viewed file has uncommitted
  changes, with an inline control to open that file in the reviewer's configured
  diff tool. `xdg-open` cannot diff a single file, so the control launches
  `git difftool`, which honours the reviewer's `diff.tool` (e.g. meld).
- No change to the working-tree query itself, to detection, or to the marked-file
  reconciliation it also feeds.
- A backend poller that re-runs the working-tree query on a fixed interval and
  emits a `worktree:changed` event when the reported status differs from the last
  poll, so the banners update live. The frontend listens for that event and
  re-fetches the status, mirroring the existing `review:changed` flow.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `working-tree-status`: adds three presentation requirements — a page-wide
  uncommitted-changes banner, a per-file warning banner that offers the configured
  diff tool, and live updates so both banners reflect a disk change without a
  manual refresh. The query and reconciliation requirements are unchanged.

## Impact

Code already present in this worktree and described (not modified) by this plan:

- `backend/open.go` — `OpenDiffTool` launches `git difftool --no-prompt -- <path>`
  detached (a GUI diff tool stays open for the review and would block the bridge
  call); the git binary is overridable for tests.
- `backend/main.go` — `OpenDiffToolForFile` binds it on `App`.
- `backend/open_test.go` — `TestOpenDiffTool` asserts the invocation shape and the
  missing-git error.
- `ts/client.ts` — `openDiffToolForFile()` exposes the bound method.
- `ts/core/worktree.ts` — pure `workingTreeBannerText` / `isFileDirty` helpers,
  DOM-free and unit-tested.
- `ts/core/worktree_test.ts` — unit tests for those helpers.
- `ts/main.ts` — caches the status, renders the two banners on load and refresh,
  and wires the difftool button.
- `backend/assets/index.html`, `backend/assets/style.css` — the two banner
  elements and a shared `.banner` base with info/warning variants; the existing
  `#review-changed-banner` is refactored onto that base.

Code this change still needs to add, for the live-update behaviour requested at
review:

- `backend/watcher.go` — a working-tree poller alongside `watchStateFile`: a ticker
  that re-runs the working-tree query and emits `worktree:changed` when the reported
  status differs from the previous poll. A pure equality helper over
  `WorkingTreeStatus` decides "differs", so it is unit-testable without the ticker.
- `backend/main.go` — start the working-tree poller in `startup`, beside the
  existing `go a.watchStateFile(ctx)`.
- `ts/main.ts` — subscribe to `worktree:changed` and re-run the existing
  `refreshWorkingTreeStatus` on each event, so both banners re-render from the fresh
  status.
- Tests: Go coverage for the status-equality helper (changed vs unchanged); a Deno
  test if the diff logic is factored into `ts/core/worktree.ts`.

## Workflow note for the reviewer

The two presentation requirements have already been appended directly to the main
spec `openspec/specs/working-tree-status/spec.md` in this worktree, ahead of the
OpenSpec change being recorded. The same requirements are captured as a delta
under this change's `specs/`. At archive time the delta would normally be folded
into the main spec; here the main spec already carries them, so reconciliation
should treat the delta as already-applied rather than re-appending it. This
deviation is flagged for the review gate; the implementation itself is unaffected.
