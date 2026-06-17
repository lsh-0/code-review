## 1. Stage 1 — Split RefreshState (perf fix, ships alone)

- [x] 1.1 In `backend/main.go`, rename the `loadDiff()` body into an exported `RecomputeDiff() error` (git diff, ParseDiff, reset file cache, reconcile AddFileDiff); keep startup calling it
- [x] 1.2 Add `ReloadReview() error` to `backend/main.go`: LoadReview into `a.review` only, no git
- [x] 1.3 Delete `RefreshState` from `backend/main.go`
- [x] 1.4 In `frontend/web.go`, change `selectFile` to render from loaded state — drop the `refreshState` call, load comments and render directly
- [x] 1.5 In `frontend/web.go`, change `selectOverview` to call `ReloadReview` (cheap) instead of the old refresh
- [x] 1.6 In `frontend/web.go`, change the Refresh button handler to call `RecomputeDiff` then `ReloadReview`, then re-render
- [x] 1.7 Add `backend/main_test.go` coverage: `RecomputeDiff` invokes the diff fetcher; `ReloadReview` does not (inject the fetcher via the existing file-cache seam)
- [x] 1.8 Build and run; confirm file selection has no perceptible pause and the Refresh button still surfaces newly committed files — verified: clicking between files is instant; Refresh works (startup/refresh git-diff slowness deferred to section 7)

## 2. Stage 2 — callBackend bridge helper

- [x] 2.1 Add `app()` to `frontend/web.go`: resolve the bound App once, return nil if the bridge is not ready
- [x] 2.2 Add `callBackend(method, out, onOK, args...)` and `reportBridgeError(method, rs)` to `frontend/web.go`
- [x] 2.3 Convert the read wrappers (`loadReviewInfo`, `loadDiffFiles`, `loadMarkedFiles`, `loadComments`, `loadCommentedFiles`, `copyStatePrompt`, `expandGap`'s GetFileLines call) to `callBackend` — `browseFile` and `expandGap` keep bespoke catches (user-facing alert / disable-affordance) but use `app()` for the guard
- [x] 2.4 Convert the mutation wrappers (`setFileMarked`, add/update/reply/resolve/ignore/reactivate/delete comment) to `callBackend`
- [x] 2.5 Build and run; confirm no behaviour change across loads, comments, marks, and expand-context — frontend compiles under js/wasm and the full frontend test suite passes (manual rebuild check is the reviewer's)

## 3. Stage 3 — Typed git-query layer and mark reconciliation

- [x] 3.1 Create `backend/gitquery.go` with `DiffQuery{RepoPath, Base, Head}` wrapping `GetDiff` + `ParseDiff`; have `RecomputeDiff` use it
- [x] 3.2 Add `WorkingTreeStatus{Modified, Deleted, DirtyFiles}` and `GetWorkingTreeStatus(repoPath)` over `git status --porcelain`, excluding untracked files
- [x] 3.3 Add a `GetWorkingTreeStatus() (string, error)` bridge method to `backend/main.go` returning the status as JSON
- [x] 3.4 Add pure `Review.ReconcileMarks(changedPaths []string)` to `model/model.go`, dropping marks for changed or deleted files
- [x] 3.5 Call `ReconcileMarks` from `RecomputeDiff` so newly changed files drop off the marked set on refresh — driven by a pure `changedSince` diff-delta (modified/added/removed files); in-session only (cross-restart change not detected, noted as a limitation)
- [x] 3.6 Add tests: `model/model_test.go` for `ReconcileMarks` (changed unmarked, unchanged kept, deleted unmarked); backend test for `GetWorkingTreeStatus` parsing (modified/deleted reported, untracked excluded, clean tree empty), plus `changedSince`

## 4. Stage 4 — Incremental comment updates

- [x] 4.1 Define `CommentMutationResult{FilePath, LineNumber, Comments, FileStatus}` as a backend wire type in `backend/main.go` (dropped `RootID`: the line anchor plus the full comment array is sufficient to address and rebuild the thread)
- [x] 4.2 Change the comment mutators (add/reply/update/resolve/ignore/reactivate/delete) to return `CommentMutationResult` JSON describing the one affected thread and the file's recomputed status — via shared `mutationResult`/`saveAndResult` helpers; line anchor from pure `model.CommentRootLine`; status from pure `model.FileCommentStatus` (also now backs the frontend pill, removing duplication)
- [x] 4.3 Give comment-thread DOM nodes a stable `data-line` attribute (`lineCommentThread`) and diff-line rows a `data-line` (in `createDiffLine`) so a new thread can be inserted after its line; `data-root` proved unnecessary
- [x] 4.4 Add `applyMutation` + `patchLineThread` + `setFileStatusPill` to `frontend/web.go`: overwrite `commentsCache[FilePath]`, re-render/insert/remove just the addressed thread, update the file pill from `FileStatus`; overview surface rebuilt wholesale (no diff state to preserve there)
- [x] 4.5 Route every comment action through `applyMutation`; removed the now-dead `refreshAfterAction` and `refreshFileView`
- [x] 4.6 Tests added where unit-testable without a DOM: `model` `FileCommentStatus` + `CommentRootLine` (the thread anchor/status logic), backend `MutationResultShape` (the wire contract). `applyMutation`/`patchLineThread` manipulate the DOM, which the GopherJS test env lacks, so their behaviour is covered by 4.7
- [x] 4.7 Build and run; confirm adding/resolving/deleting a comment preserves expanded context lines and scroll position — verified: expanded context and scroll survive comment add/resolve/delete; thread and pill update in place

## 5. Stage 5 — External-change watcher and refresh banner

- [x] 5.1 Create `backend/watcher.go` watching `a.statePath` — a backend mtime poll (no new dependency, vs fsnotify), debounced (~200ms) via the existing `bep/debounce`
- [x] 5.2 Add self-write suppression: an `App.persist()` wrapper records the state file's mtime after every GUI write (`markSaved`), and the watcher treats only a newer mtime as external. All `SaveReview(a.statePath…)` sites route through `persist`
- [x] 5.3 Wire the watcher into `startup`: `go a.watchStateFile(ctx)`; on a genuine external change, `ReloadReview`, re-baseline the mtime, then `runtime.EventsEmit(a.ctx, "review:changed")`
- [x] 5.4 Add a dismissable top-of-window banner to `assets/index.html` and `assets/style.css` (info-amber, "Changes to the review have been made. [Refresh] [×]"); `#app` made a column flex so the banner shrinks the panes rather than overlaying
- [x] 5.5 Register `EventsOn("review:changed", ...)` once via `setupReviewChangedBanner` in `initialize()`; it only shows the banner, never re-renders the view
- [x] 5.6 Wire the banner refresh to `performFullRefresh` (extracted, shared with the Refresh button) then hide the banner; wire dismiss to hide the banner and leave the view as-is
- [x] 5.7 Add a backend test (`TestStateChangedExternally`) that a GUI save is not seen as external, an external write is, and a re-baseline clears it
- [x] 5.8 Build and run; confirm an external edit (simulate by editing the JSON) raises the banner without changing the view, and that refresh absorbs it — verified: external edit raises the banner (view unchanged), Refresh absorbs it, × dismisses, and own comment actions do not raise it

## 6. Close-out

- [x] 6.1 Run all module test suites (backend, model, and the GopherJS frontend suite) and `manage.sh lint` — all green; lint tidied `strings.Split`→`SplitSeq` in gitquery.go
- [x] 6.2 Update `CHANGELOG.md` under Unreleased with the user-visible outcomes (immediate file selection, comment thread updates in place keeping context/scroll, refresh banner, mark drops on commit)
- [x] 6.3 Removed the addressed TODO.md entries (refresh-implementation review; preserve-page-state-on-comment); noted the `GetWorkingTreeStatus` groundwork on the banner item and the `durable-file-marks` follow-up on the unmark-on-commit item

## 7. Post-implementation performance analysis (deferred — not blocking)

- [ ] 7.1 Profile the ~5s startup pause: `RecomputeDiff`'s single `git diff` before the window paints, vs Wails/WebKit init. Stage 1 removed the per-click cost (file-select is now instant), so this is the remaining target.
- [ ] 7.2 Profile the ~2s Refresh pause (same `git diff` cost, now isolated to the explicit action). Consider running it off the UI path or showing progress.
