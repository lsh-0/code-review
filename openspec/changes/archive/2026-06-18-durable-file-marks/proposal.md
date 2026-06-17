## Why

The per-file "done" mark exists so the reviewer can see at a glance what still needs reviewing. For that
to hold, a marked file that changes must drop off the marked set. Today the marked set is a bare list of
paths with no record of *which version* was reviewed, so a file marked in one session, then changed by a
commit while the app is closed, is still shown as marked when the app reopens — the exact case the mark is
meant to guard against. The interim diff-delta reconciliation only catches changes within a running
session, leaving this between-session blind spot.

## What Changes

- **BREAKING (state file):** `marked_files` changes from a list of paths (`["a.go"]`) to a list of records
  carrying the path and the file's git blob SHA at mark-time
  (`[{"path":"a.go","blob":"b023cd08…"}]`).
- A file is marked with the blob SHA of its content at the review's source-branch HEAD. On opening a
  review, each mark's stored SHA is compared against the file's current SHA; a mark whose file changed
  (different SHA) or was deleted is evicted. This makes mark eviction survive app restarts.
- The trigger is committed changes only: the signature is the committed source-HEAD blob, so a new commit
  touching the file unmarks it, while uncommitted working-tree edits do not (those are surfaced by the
  separate uncommitted-changes banner, consistent with the pre-merge review scope).
- Old state files (bare-path marks) are backfilled on next open: each legacy mark's blob SHA is computed
  from the current source-HEAD and stored, establishing a baseline so the blind spot closes for existing
  marks too rather than leaving them signature-less.
- The in-session `ReconcileMarks` / `changedSince` diff-delta added as interim groundwork is removed; the
  blob-SHA comparison supersedes it (precise and restart-surviving). The `git status` working-tree query
  and `DiffQuery` are unaffected.
- The state file's `_readme` (and `statefile-usage.md`) is updated to describe the new `marked_files`
  shape so an agent unmarking a file edits the record, not a bare path.

## Capabilities

### New Capabilities

- `durable-file-marks`: How a file mark records the reviewed version (a git blob SHA), how marks are
  evicted when the file changes across sessions, and how legacy bare-path marks are migrated.

### Modified Capabilities

<!-- No existing spec's requirements change. The working-tree-status capability proposed in
     refactor-refresh-state-architecture is independent; this only removes that change's interim
     in-session ReconcileMarks, which has no spec of its own. -->

## Impact

- `model/model.go`: `MarkedFiles []string` becomes a typed list of mark records; `MarkFile`,
  `UnmarkFile`, `IsFileMarked` operate on records; `ReconcileMarks`/`changedSince` removed. A pure
  eviction function compares stored vs current SHAs.
- `backend/main.go`: `SetFileMarked` computes and stores the blob SHA at mark-time; `RecomputeDiff` (or
  startup) runs eviction with current SHAs; `RecomputeDiff` stops calling the removed diff-delta
  reconciliation. `GetMarkedFiles` returns the paths the frontend needs.
- `backend/gitquery.go` (or `git.go`): a batch blob-SHA query (`git ls-tree <rev> <paths…>`) so all marks
  are checked in one git call; `changedSince`/`diffSignature` removed.
- `backend/statefile-usage.md`: documents the new `marked_files` record shape.
- State-file schema change with on-load backfill migration; no data loss (paths preserved, signatures
  added).
- Constraints unchanged: git stays backend-side; `model` stays pure Go 1.19 with no new dependencies (the
  SHA is supplied by the backend, the model only compares strings).
