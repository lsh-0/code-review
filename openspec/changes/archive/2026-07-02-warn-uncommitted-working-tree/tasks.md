## 1. Backend: launch the configured diff tool

- [x] 1.1 `backend/open.go` exposes `OpenDiffTool(repoPath, path)` that runs `git difftool --no-prompt -- <path>` detached, with the git binary overridable via `diffToolCommand`.
- [x] 1.2 `backend/main.go` binds it on `App` as `OpenDiffToolForFile(filePath)`, resolving the path against the repository root and returning a launch failure to the caller.

## 2. Frontend: pure banner logic

- [x] 2.1 `ts/core/worktree.ts` exposes `workingTreeBannerText(status)` returning the page-wide message (with modified/deleted counts) or `null` when clean.
- [x] 2.2 `ts/core/worktree.ts` exposes `isFileDirty(status, path)` returning whether the path is in the status' `dirty_files` set.
- [x] 2.3 `ts/client.ts` exposes `openDiffToolForFile(path)` over the bound method.

## 3. Frontend: render and wire the banners

- [x] 3.1 `backend/assets/index.html` carries a page-wide `#working-tree-banner` (`.banner-info`) and a per-file `#file-dirty-banner` (`.banner-warning`) with an inline `Open in diff tool` control.
- [x] 3.2 `backend/assets/style.css` provides a shared `.banner` base with `.banner-info` / `.banner-warning` variants and a `.banner-link`; `#review-changed-banner` reuses the base.
- [x] 3.3 `ts/main.ts` caches the working-tree status, renders the page-wide banner on load and after every full refresh, renders the per-file banner on file selection (hidden on the overview), and wires the difftool button to `openDiffToolForFile` with a failure alert.

## 4. Tests

- [x] 4.1 `backend/open_test.go` `TestOpenDiffTool` asserts the `git difftool` invocation shape and the missing-git error; `go test ./...` passes, gofmt/vet clean.
- [x] 4.2 `ts/core/worktree_test.ts` covers `workingTreeBannerText` and `isFileDirty`; `deno test` passes, fmt/lint/type-check clean.

## 5. Live updates: poll the working tree and refresh the banners

This section is the work requested at review (banners updating the moment a file
changes on disk, via periodic git polling). It is not yet implemented.

- [x] 5.1 `backend/gitquery.go` exposes `WorkingTreeStatusEqual`, a pure order-insensitive equality over `WorkingTreeStatus`'s modified and deleted sets, so the poller can emit only on a real change.
- [x] 5.2 `backend/watcher.go` adds `watchWorkingTree`: a ticker (`worktreeWatchInterval`, 1000 ms) that re-runs `GetWorkingTreeStatus`, compares it against the previous poll via 5.1, and emits `worktree:changed` on a difference. It runs until `ctx` is cancelled, mirroring `watchStateFile`. A query error skips the tick, keeping the last-seen status.
- [x] 5.3 `backend/main.go` starts `go a.watchWorkingTree(ctx)` in `startup`, beside `go a.watchStateFile(ctx)`.
- [x] 5.4 `ts/main.ts` subscribes to `worktree:changed` (via `setupWorkingTreeWatcher`) and calls `refreshWorkingTreeStatus` on each event, re-rendering both banners from the fresh status.

## 6. Tests

- [x] 6.1 Go: `TestWorkingTreeStatusEqual` (`backend/gitquery_test.go`) covers equal statuses, order-insensitivity, two clean trees, a differing modified set, a differing deleted set, and the same path modified vs deleted; `go test ./...` passes, gofmt/vet clean.
- [x] 6.2 Deno: no new diff logic was factored into `ts/core/worktree.ts` — the frontend only re-fetches on the event, and the equality check lives entirely in the backend, so no new frontend test is warranted. `deno test` still passes (104), fmt/lint/type-check clean.

## 7. Validate

- [x] 7.1 Run `openspec validate warn-uncommitted-working-tree` and resolve any reported issues.
