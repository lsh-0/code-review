## Context

`working-tree-status` already provides a typed `GetWorkingTreeStatus` query that
reports tracked files modified or deleted (untracked excluded), and it is already
bound and exposed as `getWorkingTreeStatus()` in `ts/client.ts`. Nothing in the UI
reads it, so a reviewer has no signal that the committed diff they are reading may
not match the file on disk. This change consumes that query in the UI. The
external-change banner in `ts/main.ts` already establishes a show/hide banner
pattern to follow.

## Goals / Non-Goals

**Goals:**
- Make a dirty working tree visible at a glance (page-wide) and per file.
- Have both banners update on their own shortly after a file changes on disk, so
  the warning appears without the reviewer triggering a refresh.
- For a dirty file, offer the reviewer their own diff tool, since the GUI shows
  only the committed diff.
- Keep the decision logic pure and unit-testable, separate from the DOM and the
  bridge.

**Non-Goals:**
- No change to the working-tree query, to detection, or to the marked-file
  reconciliation the query also feeds.
- No surfacing of untracked (new) files — the query excludes them and that stays.
- No in-GUI diff of the working-tree change; the per-file control hands off to the
  reviewer's configured external tool rather than rendering a second diff.
- No per-file filesystem monitoring (fsnotify or a watch on each file). Liveness
  comes from periodically re-running the one working-tree query, matching how the
  review-state watcher already works.

## Decisions

**Use `git difftool`, not `xdg-open`, for the per-file control.** `xdg-open`
opens a single file in its default application; it cannot present a diff of the
working-tree change. `git difftool --no-prompt -- <path>` honours the reviewer's
`diff.tool` (e.g. meld) and shows the change against the index, which is what the
banner is warning about. `OpenDiffTool` lives beside the existing
`OpenInPreferredApp` in `backend/open.go`, and the git binary is overridable
(`diffToolCommand`) exactly as `openerCommand` is, so the invocation is testable.

**Launch the diff tool detached and do not wait on it.** A GUI diff tool stays
open for the duration of the review; waiting on it would block the bridge method
and hang the UI. `OpenDiffTool` uses `cmd.Start()` and reaps the process in a
background goroutine. The trade-off: only a failure to *start* (e.g. git absent)
is reported; a failure that surfaces after the tool launches is not observed. That
is acceptable, because the reviewer sees the tool fail to appear and the GUI stays
responsive.

**Keep the banner decision logic in a pure module.** `ts/core/worktree.ts`
exposes `workingTreeBannerText(status)` (the page-wide message, or `null` when
clean) and `isFileDirty(status, path)` (the per-file check). Both are DOM-free and
bridge-free, so they are unit-tested in isolation; `ts/main.ts` only reads them to
toggle visibility. This mirrors the project's preference for pure, composable
helpers over logic embedded in render code.

**Cache the status and refresh it with the review.** `ts/main.ts` holds the last
fetched status so the per-file check on file selection needs no fresh git call. It
is refetched on load, on every full refresh, and on each `worktree:changed` event
(below), because a new commit or a disk edit can change what is uncommitted. A
failed refetch leaves the previous status and banner in place rather than clearing
a still-valid warning.

**Update the banners live via a backend poller, not a filesystem watch.** The
reviewer asked for the warnings to appear the moment they change a file on disk.
Rather than watch individual files, a backend ticker re-runs the working-tree query
periodically and emits a `worktree:changed` event when the reported status differs
from the previous poll; the frontend re-fetches on that event and re-renders both
banners. This mirrors `watchStateFile`, which already polls the review-state file
on a ticker and emits `review:changed`, so the two watchers share one shape and one
dependency-free polling approach.

- *Backend, not frontend, owns the poll.* Watching belongs beside the existing
  state watcher, keeps the git call off the bridge, and lets the frontend stay a
  passive event subscriber (as it already is for `review:changed`). The frontend
  gains no polling loop of its own.
- *Emit only on change.* The poller compares the freshly queried status against the
  last one it saw and emits only when they differ, so a steady-state dirty (or
  clean) tree produces no event traffic and no needless re-render. The comparison is
  a pure equality over `WorkingTreeStatus` (same modified set, same deleted set),
  factored out so it is unit-testable without the ticker.
- *Interval.* The poll runs every 1000 ms (the reviewer's chosen interval),
  keeping "the moment I change something" perceptually immediate while staying one
  cheap `git status --porcelain` per tick. It lives beside `watchInterval` as its
  own `worktreeWatchInterval` constant for symmetry.
- *No debounce needed on the emit.* Unlike the state watcher, which coalesces a
  burst of writes before a relatively expensive reload, this poller only emits an
  event; the frontend's own re-fetch is cheap and idempotent, so a plain
  emit-on-change is enough. (If churn proves noticeable in practice, the same
  `debounce` helper can wrap the emit later.)

**Share one banner CSS base across all three banners.** A `.banner` base carries
layout and the `.hidden` rule; `.banner-info` (green) and `.banner-warning`
(orange) tint it. The existing `#review-changed-banner` is refactored onto the
base and keeps only its amber tint, removing duplicated layout rules.

## Risks / Trade-offs

- [Detached diff tool hides post-launch failures] → The reviewer sees no tool
  appear and the GUI stays responsive; start failures (the common case, git
  absent) are still reported. Accepted.
- [A working-tree change is now reflected within one poll interval, not only on a
  manual refresh] → The banner lag is bounded by the poll interval (1000 ms) rather
  than open-ended; the reviewer no longer has to refresh to see a fresh warning.
- [The poller adds one `git status --porcelain` per interval for the app's
  lifetime] → It is the same shape and roughly the same cost as the existing
  state-file poll, and `git status` is cheap; accepted as the price of live
  updates. Emitting only on change keeps downstream work (the frontend re-fetch and
  re-render) to actual transitions.
- [The presentation requirements were appended to the main spec ahead of this
  change being recorded] → Flagged in the proposal for the review gate;
  reconciliation should treat the delta as already-applied. No code impact.

## Migration / Rollout

No migration. The banners are additive and hide on a clean tree, so a repository
with no uncommitted changes sees no UI change. `review.js` is a gitignored bundle
artefact; the running app reflects the change only after a rebuild via
`manage.sh`.
