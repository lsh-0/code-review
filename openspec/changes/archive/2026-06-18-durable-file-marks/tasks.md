## 1. Model: mark records and pure eviction

- [x] 1.1 Add `FileMark{Path, Blob}` to `model/model.go`; change `Review.MarkedFiles` to `[]FileMark` (json `marked_files`)
- [x] 1.2 Port `IsFileMarked`, `MarkFile`, `UnmarkFile` to operate on records; `MarkFile` takes the path and blob SHA
- [x] 1.3 Add custom JSON unmarshal so `marked_files` accepts both the legacy `[]string` and the new `[]FileMark`, mapping legacy paths to `FileMark{Path, Blob:""}`
- [x] 1.4 Add pure `Review.EvictChangedMarks(current map[string]string)`: keep when SHA matches, backfill when stored blob is empty, evict when path absent (deleted) or SHA differs
- [x] 1.5 Remove `ReconcileMarks`, and remove its callers' expectations (model side)
- [x] 1.6 Tests in `model/model_test.go`: unchanged stays, changed evicts, deleted evicts, legacy backfills-and-stays, then backfilled-then-changed evicts; plus unmarshal of both legacy and new shapes

## 2. Backend: blob query and wiring

- [x] 2.1 Add `BlobSHAs(repoPath, rev string, paths []string) (map[string]string, error)` over `git ls-tree <rev> -- <paths…>` parsing sha/path; absent paths omitted
- [x] 2.2 In `SetFileMarked`, compute the file's blob SHA at the source-branch HEAD and pass it to `MarkFile`
- [x] 2.3 In `RecomputeDiff` (or startup), query current SHAs for the marked paths and call `EvictChangedMarks`; remove the `ReconcileMarks`/`changedSince` call
- [x] 2.4 Remove `changedSince` and `diffSignature` from `backend/gitquery.go` and their tests; keep `DiffQuery` and `GetWorkingTreeStatus`
- [x] 2.5 Confirm `GetMarkedFiles` still returns the bare paths the frontend expects (extract `.Path` from the records)
- [x] 2.6 Tests: `BlobSHAs` against a real repo (sha returned, deleted path absent); a `SetFileMarked` test that the stored mark carries a non-empty blob; a `RecomputeDiff` test that a committed change to a marked file evicts it

## 3. Agent instructions

- [x] 3.1 Update `backend/statefile-usage.md` so the embedded `_readme` documents the `marked_files` record shape (path + blob) and that unmarking removes the matching record
- [x] 3.2 Update the `GetStatePrompt` text in `backend/main.go` if it references removing a bare path from `marked_files` — no change needed: it says "remove it from `marked_files`" without assuming a bare path; the record shape lives in the `_readme`

## 4. Verify

- [x] 4.1 Run model, backend, and frontend test suites and `manage.sh lint`
- [x] 4.2 Build and run: mark a file, commit a change to it, reopen the app, confirm it is unmarked; mark a file, make only an uncommitted edit, confirm it stays marked; open a pre-existing (legacy bare-path) state file and confirm marks survive the first open — verified all three: restart eviction, uncommitted stays marked, legacy marks migrate and survive
- [x] 4.3 Update `CHANGELOG.md` under Unreleased (marks now survive restarts and drop when a commit changes the file; `marked_files` schema change)