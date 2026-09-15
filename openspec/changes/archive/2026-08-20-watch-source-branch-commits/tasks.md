## 1. Git query

- [x] 1.1 Add `CommitSHA(repoPath, rev string) (string, error)` to `backend/gitquery.go`, beside
      `BlobSHAs`: runs `git -C <repo> rev-parse --verify <rev>^{commit}`, returns the trimmed SHA, and
      wraps errors in the file's existing style (including `*exec.ExitError` stderr). Comment why
      `--verify` and `^{commit}` are used.
- [x] 1.2 Add `TestCommitSHA` to `backend/gitquery_test.go` using `setupTestRepo(t)`: a valid ref
      resolves to a 40-char SHA; the SHA changes after a new commit; an unknown ref returns an error
      rather than echoing the ref back.

## 2. Poll interval

- [x] 2.1 In `backend/watcher.go`, replace `watchInterval` (500ms) and `worktreeWatchInterval`
      (1000ms) with a single shared 5000ms `watchInterval`, and point both existing pollers at it.
      Rewrite the constants' comments: the rationale is now that a poller only runs while nothing is
      being reported, so per-subject tuning buys nothing. Keep `watchDebounce` and its comment.

## 3. Suspend gate

- [x] 3.1 Add per-watcher suspend flags to `App` in `backend/main.go`, guarded by a mutex in the style
      of `savedMu`/`lastSavedMtime`, with a comment saying the flags are set from the frontend and read
      from the watcher goroutines.
- [x] 3.2 Add `SuspendWatch(watcher string)` and `ResumeWatch(watcher string)` bridge methods setting
      those flags. `ResumeWatch` must also clear the relevant poller's baseline so the next tick
      re-seeds rather than re-reporting the change just cleared.
- [x] 3.3 Gate `watchStateFile` and `watchSourceBranch` on their flag: while suspended, skip the read
      entirely.
- [x] 3.4 Gate `watchWorkingTree` differently, per design Decision 3: while suspended it must still
      read, because the return to clean is its resume condition. Suspended means "do not report a new
      dirty state"; a transition to clean is still emitted and clears the suspension. Comment this as
      the one watcher whose gate is not a plain skip.
- [x] 3.5 Add tests to `backend/watcher_test.go` for the gate logic, testing the predicates rather than
      the goroutines (as `TestStateChangedExternally` does): suspended suppresses a report; resume
      re-seeds so the cleared change is not re-reported; a suspended watcher does not affect another's
      flag; and for the working tree specifically, the dirty → dirtier → clean sequence reports the
      first dirty state and the clean state but not the intermediate one.

## 4. Commit baseline

- [x] 4.1 Add the baseline value type to `backend/watcher.go`: holds the last-seen SHA and exposes a
      method that, given a freshly read SHA, reports whether it is a change and adopts it. An empty
      baseline means "not yet seeded" — it adopts and reports no change, which also makes it the
      re-seed mechanism used by `ResumeWatch`. Comment why this differs from `watchWorkingTree`'s
      zero-value fallback (an empty SHA is not a meaningful value, so comparing against it would raise
      a spurious startup banner).
- [x] 4.2 Add baseline tests to `backend/watcher_test.go`: first read seeds and reports nothing; an
      unchanged read reports nothing; a differing read reports once and does not report again; a
      cleared baseline re-seeds silently.

## 5. Commit poller

- [x] 5.1 Add `watchSourceBranch(ctx)` to `backend/watcher.go`. Each tick, unless suspended, reads
      `CommitSHA(a.repoPath, a.review.SourceBranch)`; a read error `continue`s, leaving the baseline
      intact; a reported change emits `commits:changed` with no payload and suspends the watcher.
      Runs until `ctx` is cancelled. Doc comment in the style of its two neighbours, stating that the
      poller only detects — it does not reload or recompute.
- [x] 5.2 Start it from `startup` in `backend/main.go`, next to `go a.watchStateFile(ctx)` and
      `go a.watchWorkingTree(ctx)`, with a one-line comment matching theirs.

## 6. Uncommitted-changes banner text

- [x] 6.1 Change `workingTreeBannerText` in `ts/core/worktree.ts` to return a fixed sentence reporting
      that uncommitted changes exist, with no counts. Keep its signature, its `WorkingTreeStatus`
      input, and its null-for-a-clean-tree contract, so `main.ts`'s call site is unchanged. Update its
      doc comment: it reports a binary condition, and the per-file detail belongs to the file list and
      the per-file banner.
- [x] 6.2 Update the three `workingTreeBannerText` tests in `ts/core/worktree_test.ts`: the two
      counting cases become one asserting the same message for differing dirty sets; the clean-tree
      null case is unchanged. Leave the `isFileDirty` tests alone.
- [x] 6.3 Confirm nothing else reads the counts for display: `WorkingTreeStatus.Modified`/`Deleted`
      stay on the backend type (they derive `DirtyFiles`, and `WorkingTreeStatusEqual` must keep
      comparing both sets so the poller still detects file-level transitions).

## 7. Refresh banner text

- [x] 7.1 Add `reviewChangedBannerText` to `ts/core/` (new module, or beside the existing pure
      helpers), mapping the set of pending change sources (`"commits"`, `"state"`) to the banner
      sentence: commits only, state only, or both. Pure and DOM-free, mirroring `workingTreeBannerText`.
- [x] 7.2 Add its unit test covering all three populated cases and the empty set.
- [x] 7.3 In `backend/assets/index.html`, replace the hardcoded sentence inside
      `#review-changed-banner` with an identified span the frontend fills, as
      `#working-tree-banner-text` already is.

## 8. Frontend wiring

- [x] 8.1 In `ts/main.ts`, hold the pending change-source set; on `review:changed` add `"state"`, on
      the new `commits:changed` add `"commits"`, and in both cases render the banner text from
      `reviewChangedBannerText`, show the banner, and call `SuspendWatch` for the watcher that fired.
- [x] 8.2 On both refresh and dismiss, clear the pending set and call `ResumeWatch` for every watcher
      the cleared banner represented. Verify the existing refresh path (`triggerRefresh` →
      `performFullRefresh`) is otherwise unchanged — it already recomputes the diff, re-anchors
      comments, and evicts stale marks.
- [x] 8.3 Wire the working-tree resume: when `refreshWorkingTreeStatus` observes a clean tree, resume
      that watcher. Confirm the banner still hides on clean, as now.

## 9. Verify

- [x] 9.1 Run the Go tests (`go test ./...` in `backend`) and the frontend tests; both suites pass.
- [x] 9.2 Manually confirm end to end: with the GUI open on a review, commit to the source branch from
      a terminal — the refresh banner appears within ~5s reporting new commits, the displayed diff is
      unchanged until Refresh is pressed, and Refresh then shows the new commit's files. Repeat with a
      clean working tree before and after the commit (e.g. `git commit --amend --no-edit`) to confirm
      detection does not depend on a working-tree transition.
- [x] 9.3 Manually confirm the gate: after the banner is raised, a second commit produces no further
      alert; after Refresh, a third commit does raise it again; after Dismiss (rather than Refresh), a
      further commit also raises it again, and the dismissed one is not re-reported.
- [x] 9.4 Confirm no spurious banner at startup, and none on a steady branch left idle.
- [x] 9.5 Confirm the uncommitted-changes banner: it reports no counts, reads the same for one dirty
      file as for several, still disappears when the tree returns to clean, and the per-file orange
      banner still appears on a dirty file and offers the diff tool.
- [x] 9.6 At sync/archive time, widen the `external-change-notification` spec's Purpose paragraph from
      "changed the review state file" to cover both triggers (an external state-file write and a new
      commit on the source branch).
