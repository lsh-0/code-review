## Context

The per-file "done" mark is a revisit-tracker: a marked file is one the reviewer has finished with, so
the marked set tells them what still needs attention. For that signal to stay true, a marked file that
changes must drop off the set. Today `model.Review.MarkedFiles` is a `[]string` of paths
(`"marked_files": ["a.go"]`), carrying no record of which version was reviewed, so eviction can only be
inferred by comparing diffs — which the interim Stage 3 work in `refactor-refresh-state-architecture`
does within a single running session via `ReconcileMarks`/`changedSince`. That leaves a between-session
blind spot: mark a file, close the app, a commit changes the file, reopen — it is still marked.

Git content-addresses every file, so the blob SHA at the source-branch HEAD
(`git rev-parse <sourceBranch>:<path>`, or `git ls-tree <rev> <paths…>` for many at once) is a cheap,
exact, restart-surviving signature of the reviewed version. The review compares the source branch against
the base branch, and marks mean "I reviewed this committed version," so the committed source-HEAD blob is
the right signal — it reacts to new commits, not to uncommitted edits (which are out of the pre-merge
review scope and surfaced separately by the uncommitted-changes banner).

## Goals / Non-Goals

**Goals:**

- A mark stores the file's blob SHA at mark-time; on open, a differing or missing SHA evicts the mark.
- Eviction survives application restarts, closing the between-session blind spot.
- Legacy bare-path state files are migrated by backfilling signatures on first open, without dropping the
  existing marks.
- All marks are checked in one git call, not one per file.
- Remove the now-redundant in-session `ReconcileMarks`/`changedSince`; keep `DiffQuery` and the
  working-tree status query untouched.
- `model` stays pure: it compares SHA strings supplied by the backend and never shells out.

**Non-Goals:**

- No reaction to uncommitted working-tree edits (committed-only trigger, by decision).
- No change to how marks are displayed or toggled in the UI; the frontend still receives a list of marked
  paths.
- No broader state-file schema rework beyond `marked_files`.

## Decisions

### Mark record shape

`MarkedFiles []string` becomes `MarkedFiles []FileMark` where:

```go
type FileMark struct {
    Path string `json:"path"`
    Blob string `json:"blob"` // git blob SHA at the source-branch HEAD when marked; "" = legacy, awaiting backfill
}
```

Serialised: `"marked_files": [{"path":"a.go","blob":"b023cd08…"}]`. An array of records (not a
path-keyed map) preserves order and stays close to the existing JSON. *Alternative considered:* the
`{filename: {revision}}` map sketch — rejected because a map loses ordering and reads awkwardly for a set
whose elements are naturally records.

### Backfill via custom JSON unmarshal

`marked_files` may be the old `[]string` or the new `[]FileMark`. A custom `UnmarshalJSON` on the mark
list (or on `Review`) tries the object form, and on failure falls back to decoding a `[]string`, mapping
each path to `FileMark{Path: p, Blob: ""}`. An empty `Blob` flags a legacy mark. On first open the backend
fills empty blobs from the current source-HEAD SHA and treats them as the baseline (never evicting a mark
solely for having had an empty blob). *Alternative considered:* a one-shot migration script — rejected;
on-load tolerance is simpler and needs no separate run.

### Eviction is pure in `model`, signatures come from the backend

The backend computes current SHAs (git) and hands `model` a `map[path]currentSHA`. A pure
`Review.EvictChangedMarks(current map[string]string)` keeps a mark when `current[path] == mark.Blob`,
backfills when `mark.Blob == ""` (set it to `current[path]`, keep), and evicts when the path is absent
from `current` (deleted) or the SHA differs. This mirrors how `ReconcileMarks` was already pure and
testable without git. *Alternative considered:* eviction reading git inside `model` — impossible, `model`
is GopherJS-compiled and cannot exec.

### Batch blob query

`BlobSHAs(repoPath, rev string, paths []string) (map[string]string, error)` runs
`git ls-tree <rev> -- <paths…>` and parses `mode SP type SP sha TAB path` lines into a map; paths absent
from the output are deleted at that rev and simply missing from the map (→ evicted). One git call covers
every mark.

### Removing the interim reconciliation

`Review.ReconcileMarks`, `changedSince`, and `diffSignature` (added in the refresh-architecture change)
are deleted, along with their tests and the `RecomputeDiff` call into them. `RecomputeDiff` instead has
the backend run `EvictChangedMarks` with freshly queried SHAs. `DiffQuery`, `GetWorkingTreeStatus`, and
their tests remain.

### `_readme` update

`statefile-usage.md` is updated so the embedded `_readme` documents the record shape, since an agent that
"unmarks any file it changes" must now remove the matching record rather than a bare string.

## Risks / Trade-offs

- **Schema break for hand-written state files** → an agent or user editing `marked_files` by hand must use
  the new record shape. Mitigation: the custom unmarshal still accepts the old `[]string`, and the
  `_readme` documents the new shape; backfill upgrades on next GUI save.
- **A mark made before the file was committed** → `git rev-parse <sourceBranch>:<path>` fails for a path
  not yet committed at that rev. Mitigation: treat a failed lookup as "no current SHA" → the path is
  absent from the map; for a freshly marked file this should not happen (it is in the diff, hence
  committed), but if it does the mark evicts, which is the safe direction.
- **Backfill baseline hides one change** → if a legacy-marked file was already changed before the upgrade
  open, backfill adopts the current SHA as baseline and will not flag that pre-upgrade change. Mitigation:
  accepted; this is the one-time cost of migrating without a prior signature, and only affects the first
  open after upgrade.
- **Ordering of eviction vs. diff recompute** → eviction needs the source-HEAD SHAs, independent of the
  diff. Mitigation: run eviction in `RecomputeDiff` after the SHAs are queried; it does not depend on the
  parsed diff.

## Migration Plan

Single change, landing after the current refresh-architecture stages. Steps: (1) add `FileMark` and the
custom unmarshal in `model`; (2) port `MarkFile`/`UnmarkFile`/`IsFileMarked` to records and add
`EvictChangedMarks`; (3) remove `ReconcileMarks`/`changedSince`/`diffSignature` and their wiring; (4) add
`BlobSHAs` and call eviction from `RecomputeDiff`; (5) compute and store the SHA in `SetFileMarked`;
(6) update `statefile-usage.md`; (7) tests. No data migration job — on-load unmarshal backfills. Rollback
is a revert; old state files still load (the unmarshal is additive), and new-shape files would need a
manual downgrade only if reverted, which is acceptable pre-1.0.

## Open Questions

- Whether to also expose a backend `GetMarkedFiles` returning records (for a future "marked against
  revision X" UI affordance) or keep returning bare paths to the frontend. Default: keep bare paths; the
  frontend has no use for the SHA yet.
