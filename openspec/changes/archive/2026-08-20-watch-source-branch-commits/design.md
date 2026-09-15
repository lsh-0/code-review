## Context

`backend/watcher.go` runs two independent pollers, both started from `startup` and both living until
the app context is cancelled:

| poller | interval | reads | emits | frontend reaction |
| --- | --- | --- | --- | --- |
| `watchStateFile` | 500ms, 200ms debounce | mtime of the review JSON, vs `lastSavedMtime` | `review:changed` | raise the dismissable refresh banner |
| `watchWorkingTree` | 1000ms | `git status --porcelain` | `worktree:changed` | silently re-fetch status, repaint the two uncommitted-change banners |

Nothing reads the source branch's HEAD after startup. `git.go` resolves the branch *name* once via
`rev-parse --abbrev-ref HEAD`; `RecomputeDiff` runs at startup and on explicit refresh only. So a
commit made on the source branch leaves the diff, file list, and marked-file set on screen
describing a revision that is no longer HEAD, with no prompt.

The partial overlap is worth naming, because it is why the gap is easy to miss: committing usually
also flips `git status` from dirty to clean, so `watchWorkingTree` often *does* fire around a commit —
but it only repaints the uncommitted-change banners. It never recomputes the diff and never raises the
refresh banner. And a commit made from an already-clean tree (rebase, `commit --amend`, cherry-pick,
merge, or an agent that stages and commits atomically) produces no working-tree transition at all, so
even that incidental signal is absent.

The constraint that shapes everything below: the reviewer is never interrupted. This tool is built for
an async round-trip — comment, hand off to an agent, come back — so the failure to avoid is a silently
stale view, and the fix is a prompt, never a reload.

## Goals / Non-Goals

**Goals:**

- Detect a source-branch HEAD move while the GUI is open, and raise the existing refresh banner.
- Reuse the existing banner element, controls, and full-refresh path rather than adding a parallel
  notification mechanism.
- Tell the reviewer *why* the banner is up, now that it has two possible causes.
- Match the conventions of the two existing pollers so `watcher.go` stays one readable pattern.

**Non-Goals:**

- Auto-refreshing on a new commit. Reviewer-controlled refresh is the whole point of the banner.
- Watching the target branch (see Decision 5).
- Showing *what* changed (commit subjects, a count of new commits, an author). The banner is a prompt,
  not a changelog; adding data means a second git query per tick for something the refresh reveals
  anyway.
- Replacing the polls with fsnotify (see Decision 9).

## Decisions

### 1. A third independent poller, not a tick added to `watchWorkingTree`

`watchWorkingTree` already runs a git subprocess every second, so folding a `rev-parse` into its tick
would be cheaper. Rejected: the `working-tree-status` spec requires that the working-tree query and the
committed-diff query "remain separate operations and SHALL NOT be fused" — a rule that exists because
`RefreshState` once fused state-reload and diff-recompute and had to be pulled apart (see the archived
`refactor-refresh-state-architecture` change). Commit-watching is on the *committed* axis; fusing it
into the working-tree poller would re-make the same mistake for a saving of one cheap subprocess.

New function `watchSourceBranch(ctx)` in `backend/watcher.go`, started from `startup` alongside the
other two.

### 2. One shared 5000ms interval for all three pollers

All three watchers move to a single `watchInterval` of 5000ms, replacing the current 500ms state-file
poll and 1000ms working-tree poll. This is a deliberate reversal of the existing per-subject tuning:
the three constants are collapsed into one.

The rationale is that under Decision 3 a poller runs only while nothing is being reported. Once it has
alerted, it is idle until the reviewer clears the alert, so a shorter interval no longer buys
responsiveness across a session — it only buys latency on the single transition from quiet to alerted.
For a tool built around the reviewer being *away* during that transition, five seconds is not
perceptible, and the steady-state cost drops from six git subprocesses per five seconds to two.

The debounce on the state-file watcher is kept. It coalesces a burst of agent writes into one reload,
and at a 5s interval a burst is more likely, not less, to be caught mid-flight.

Alternative considered: keep the tuned intervals and only add the new poller at 2000ms (the previous
version of this decision). Superseded — with the suspend gate in place the tuning is optimising an
interval that is mostly not running.

### 3. A watcher suspends once it has alerted, and resumes when its alert clears

A watcher's job is to watch for changes and alert on them. Once it has alerted, that job is done until
the alert goes away: continuing to poll re-reports a condition the reviewer has already been told
about, and — for the state-file watcher, which reloads before emitting — keeps doing real work behind a
prompt the reviewer has not yet acted on. So each poller gates on whether its own alert is currently
raised.

The gate is per-watcher, not global. A new commit suspends the commit watcher; it does not blind the
state-file watcher to an agent's replies arriving a moment later. Each alert clears independently.

What "alert cleared" means differs per watcher, because their alerts differ:

- **`watchStateFile` and `watchSourceBranch`** share the refresh banner. Their alert is cleared when
  the reviewer refreshes *or* dismisses it — both remove the banner, and both mean "I have seen this".
  Dismiss resuming the poll is what keeps detection from dying silently for the rest of the session:
  dismissing declines *this* refresh, it does not decline all future notice.
- **`watchWorkingTree`** has no buttons; its alert is the uncommitted-changes banner itself. It is
  cleared when the tree returns to clean. So the watcher suspends while the tree is dirty and resumes
  when it is clean again — which is coherent, because a dirty tree is a *steady state* the reviewer can
  already see, not news. The one thing this must not lose is the transition back to clean, which is
  what un-suspends it; see the risk below. Decision 11 removes the banner's file counts, which is what
  makes suspending it lossless rather than merely acceptable.

Mechanically the frontend owns the gate, since it owns the banners and therefore knows when an alert is
up. Two new bridge methods (`SuspendWatch`/`ResumeWatch`, keyed by watcher) let it tell the backend,
and each poller checks a guarded flag on its tick and skips the read while suspended. The ticker keeps
running rather than being stopped and recreated — a skipped tick is a branch on a boolean, and it keeps
the goroutine's shape identical to today.

Resuming re-seeds rather than comparing against the pre-alert baseline. After a refresh the reviewer
has adopted the new state, so that state is the new baseline; comparing against the old SHA would
re-alert immediately for the change just absorbed. After a dismiss the same holds for a different
reason — they have declined that change, and re-raising the banner for it on the next tick would make
Dismiss useless. Re-seeding on resume gives both the right behaviour with one rule.

### 4. Baseline seeding differs from `watchWorkingTree`, deliberately

`watchWorkingTree` seeds its baseline before the loop and, on a seed error, falls back to the zero
`WorkingTreeStatus{}` — safe there, because an empty status is a meaningful value ("clean") and the
worst case is one spurious repaint of a banner.

A zero SHA is *not* a meaningful value: if the seed failed and the baseline were left `""`, the first
successful tick would compare a real SHA against `""`, see a difference, and raise the banner at
startup for no reason. So `watchSourceBranch` treats an empty baseline as "not yet seeded": a
successful read into an empty baseline records it and emits nothing. Every later differing read emits
once and adopts the new SHA as the baseline — the same "adopt so it is not reported again" move
`watchStateFile` makes with `markSaved`.

This asymmetry is the one place the new poller does not simply mirror its neighbour, so it gets a
comment in the code saying why.

To make that logic testable the way `stateChangedExternally` is — `watcher_test.go` tests the
predicate, never the goroutine — the seed-and-compare rule lives in a small value type
(a baseline holding the last-seen SHA, with a method reporting whether a freshly read SHA is a change
and adopting it if so). The goroutine is then just ticker + `CommitSHA` + that method, and the
seeding, no-change, change-once, and error-skip cases are all unit-testable with no timing and no git.

### 5. `CommitSHA` as a typed query in `gitquery.go`

`gitquery.go` holds the typed queries the app consumes (`DiffQuery`, `BlobSHAs`,
`GetWorkingTreeStatus`); `git.go` holds the lower-level plumbing. `CommitSHA(repoPath, rev)` is a
query the poller consumes, so it belongs beside `BlobSHAs` — which it complements neatly: `BlobSHAs`
answers "did this file's content change", `CommitSHA` answers "did the branch move".

Implementation: `git -C <repo> rev-parse --verify <rev>^{commit}`, output trimmed. `--verify` makes an
unknown or ambiguous revision a clean error rather than git echoing the argument back as if it were a
SHA; `^{commit}` pins the result to a commit even if the ref is an annotated tag. Errors follow the
file's existing shape, including stderr from `*exec.ExitError`.

### 6. Watch the source branch only

The diff is three-dot: `git diff -M base...head`, i.e. merge-base(base, head) → head. Ordinary
advances on master therefore do not change what the reviewer sees — the merge base is unchanged, so
the diff is unchanged. Watching the target branch would cost a second subprocess per tick to raise
banners for changes that mostly are not changes. The cases where the merge base *does* move (the
source branch is rebased, or master is merged into it) all move the source branch's HEAD too, so this
watch already covers them.

### 7. New event `commits:changed`, not a reused `review:changed`

Emitting `review:changed` for a commit would be the smallest diff, but the state-file watcher reloads
review state before emitting, and that association ("this event means state was reloaded") is worth
keeping true. A distinct `commits:changed` also lets the frontend say which cause raised the banner,
and matches the existing `<subject>:changed` naming. The event carries no payload, like its two
siblings — the frontend reacts by showing a banner, not by reading data off the event.

The backend emits without reloading or recomputing anything: the poller's whole job is detection.

### 8. Banner text from a pure helper over a pending-source set

The banner now has two causes and can have both pending at once. The frontend keeps a small set of
pending sources (`"commits"`, `"state"`), adds to it on each event, and clears it when the reviewer
refreshes or dismisses. A pure `reviewChangedBannerText(sources)` in `ts/core/` maps that set to the
sentence, mirroring `workingTreeBannerText` in `ts/core/worktree.ts` — pure, DOM-free, unit-testable,
with `main.ts` doing only the rendering. Three messages:

- commits only: *New code was committed to this branch.*
- state only: *Changes to the review have been made.* (the current wording, now correctly scoped to
  what it actually described)
- both: *New code was committed and changes to the review have been made.*

This requires `index.html`'s hardcoded `<span>` inside `#review-changed-banner` to become an
identified, frontend-filled span, exactly as `#working-tree-banner-text` already is.

Alternative considered: keep one fixed generic sentence and skip the set entirely. Rejected — the two
causes call for different reviewer responses (a new commit invalidates the diff and the marked files; a
state edit is an agent's replies to read), and the tool's stated bias is utility over minimalism.

### 9. No change to the refresh path

`performFullRefresh` → `RecomputeDiff` + `ReloadReview` already does everything a new commit needs:
re-runs the diff, resets the file-content cache, evicts marks whose blob SHA moved, and re-anchors
comments. This change is detection and notification only.

### 10. Polling, not fsnotify

Considered and rejected. `git rev-parse <branch>` hides a resolution chain, and watching files instead
means reimplementing it:

- The ref may be a loose file at `.git/refs/heads/<branch>`, or it may be inside `.git/packed-refs`.
  `git gc`/`git pack-refs` moves it from the former to the latter and **deletes the loose file**, at
  which point a watch registered on that path is silently dead. Packing happens as part of routine
  automatic `gc`, so this is a normal occurrence, not an edge case.
- **This project's own worktrees make it worse.** Every kanban item runs in a git worktree, where
  `.git` is a *file* containing a `gitdir:` pointer, not a directory. This item's worktree resolves to
  `/mnt/WELLES/alice/dev/go/code-review/.git/worktrees/<item-id>`, and the branch ref lives in the
  *main* checkout's `refs/heads/`, shared across all the item branches. A watch on
  `<repoPath>/.git/refs/heads/...` finds nothing at all. Since the board gives every item a worktree,
  the worktree case is the normal case here — so fsnotify would have to parse the `gitdir:` pointer and
  follow the `commondir` indirection, reimplementing git's repository-discovery rules.
- It would need a new dependency (`fsnotify` is not in `go.mod`, which holds only `debounce`, `pflag`
  and `wails`), against both the project's preference for built-ins and the existing watchers' stated
  "dependency-free" rationale.
- Platform semantics differ (inotify / kqueue / ReadDirectoryChangesW), against the portability goal;
  inotify additionally has per-user watch limits.
- It would not even remove the git call: an event says a file changed, not what the SHA is, so every
  event is followed by the same read the poll does.

What it buys is sub-second latency on a prompt the reviewer is by definition not waiting for, plus the
removal of one cheap subprocess per interval. Under Decisions 2 and 3 that subprocess runs every five
seconds and only while no alert is up. `rev-parse` is the thing that already knows where the ref lives,
in every layout, for free.

### 11. The uncommitted-changes banner drops its file counts

`workingTreeBannerText` currently renders "Uncommitted changes detected: 2 modified, 1 deleted." It
becomes a fixed sentence reporting only that uncommitted changes exist.

This started as the fix for the one real cost of Decision 3 — a suspended working-tree watcher stops
re-reporting dirty states, so the counts would drift behind further edits while the banner is up. But
removing them is the better design independently of the gate: the counts are the *page-wide* banner
reporting per-file detail, which is the per-file banner's and the file list's job. The number is not
actionable on its own. A reviewer who wants to know which files are dirty looks at the files.

With the counts gone the banner is a binary condition indicator, and a binary condition cannot go
stale while it holds. Suspending the watcher becomes lossless rather than a trade-off, and the two
decisions fit rather than one paying for the other.

Scope is narrow: only the sentence changes. `WorkingTreeStatus` still carries `Modified` and `Deleted`
(they drive `DirtyFiles`, hence `isFileDirty` and the per-file banner), the backend query is untouched,
and `WorkingTreeStatusEqual` still compares both sets — the poller's change detection must stay
file-level, or edits after the first would not register as transitions at all. The counts stop being
*rendered*; they do not stop being *computed*.

## Risks / Trade-offs

- **The working-tree watcher suspends while the tree is dirty, so further edits are not re-reported
  while the banner is up.** → Made lossless by Decision 11: with no counts, the banner's claim
  ("uncommitted changes exist") stays true for every dirty state, so there is nothing to go stale. The
  transition that matters — back to clean — is the resume condition and must be evaluated even while
  suspended. Implementation note: for this watcher "suspend" means "stop reporting new dirty states",
  not "stop reading". It is the one watcher whose gate is not a plain skip, and it needs a test for the
  dirty → dirtier → clean sequence.
- **Removing the counts loses information the reviewer previously had at a glance.** → Accepted, and
  the point: it was page-wide furniture reporting a per-file fact. The file list and per-file banner
  carry it more usefully, and `dirty_files` still backs them. Reversible in one function if it is
  missed.
- **Detection latency rises to 5s, and to 5s *after* the alert clears for a second change.** → The
  reviewer is away during the first, and present and acting during the second. Neither is a wait they
  experience.
- **A commit made from a dirty tree fires two watchers, both of which then suspend.** → Correct: they
  report different things (the diff is stale; the tree is now clean). Their gates are independent, so
  clearing one does not resume the other.
- **A rapid burst of commits (a rebase replaying many commits) emits once, then suspends.** → This is
  now strictly better than before: the first commit raises the banner and the rest are absorbed while
  suspended. The re-seed on resume picks up wherever the rebase finished.
- **A detached HEAD or a source branch deleted mid-session makes every read fail.** → Reads are skipped
  and the baseline retained, so the GUI degrades to its current behaviour (no commit prompt) rather
  than flapping the banner.
- **The frontend owns the gate, so a frontend bug could leave a watcher suspended forever.** → The
  resume points are few and all sit on paths the reviewer exercises constantly (refresh, dismiss, tree
  going clean). Worth an explicit test per resume path rather than a backend failsafe timer, which
  would quietly reintroduce the polling this change removes.
- **Widening `external-change-notification` means one spec now covers two detection mechanisms.** → The
  alternative (a new capability for commit-watching) splits one banner's behaviour across two specs,
  which is worse. The capability was always about "the review changed underneath you"; the state file
  was just its only trigger. Its Purpose paragraph is widened to say so at sync time.

## Open Questions

- The exact banner wording is a judgement call, not a constraint — worth the reviewer's eye at the
  review gate.
- 5000ms is reasoned but arbitrary; trivially tunable if it feels slow in use.

Resolved: whether the working-tree banner's counts going stale under the suspend gate was acceptable.
Decision 11 removes the counts, so the question no longer arises.
