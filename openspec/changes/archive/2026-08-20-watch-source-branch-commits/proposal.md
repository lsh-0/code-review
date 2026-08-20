## Why

The GUI already tells the reviewer when the review has changed underneath them — but only when the
change arrives via the review-state JSON. An agent (or the reviewer) making a new commit on the
source branch silently invalidates the diff on screen: the file list, the hunks, and the marked-files
set all describe a revision that no longer exists, and nothing prompts a refresh. That is precisely
the stale-view failure the async round-trip this tool is built around is meant to avoid — the reviewer
hands work to an agent, comes back, and has no signal that what they are reading is out of date.

## What Changes

- Add a third backend poller that watches the source branch's HEAD commit SHA and, when it moves,
  emits a `commits:changed` event. It follows the conventions of the two existing pollers: a poll
  rather than fsnotify, a baseline seeded on first read, a failed tick skipped without disturbing the
  baseline, and a payload-free event.
- Slow all three pollers to a single shared 5s interval, replacing the current 500ms state-file and
  1000ms working-tree intervals.
- Suspend each watcher once it has raised its alert, resuming only when that alert is cleared — by
  refresh or dismiss for the refresh banner, and by the tree returning to clean for the
  uncommitted-changes banner. A watcher's job is to alert on a change, not to keep re-reporting one the
  reviewer has already been shown. Suspension is per watcher, so one raised alert never blinds the
  others.
- Drop the file counts from the page-wide uncommitted-changes banner, which becomes a fixed sentence
  reporting only that uncommitted changes exist. The counts were the page-wide banner reporting a
  per-file fact that the file list and per-file banner already carry; removing them also makes
  suspending that watcher lossless, since a binary condition cannot go stale while it holds.
- Add a typed `CommitSHA` git query alongside the existing `BlobSHAs` / `GetWorkingTreeStatus`
  queries, resolving a revision to its commit SHA.
- Raise the **existing** dismissable refresh banner on `commits:changed`, with its existing Refresh
  and Dismiss controls and its existing full-refresh behaviour. Nothing reloads underneath the
  reviewer; the diff on screen is untouched until they act.
- Make the banner say which kind of change it is reporting — new commits, external review-state edits,
  or both — via a pure text helper, rather than the current single fixed sentence. The banner element,
  its controls, and its semantics are unchanged.

Explicitly **not** in scope:

- Watching the target branch. The diff is a three-dot `base...head` diff, so it is taken from the
  merge base; ordinary advances on master do not change what the reviewer is looking at.
- Auto-refreshing on a new commit. The reviewer-controlled refresh is the point of the banner.

## Capabilities

### New Capabilities

None. This extends an existing capability rather than introducing one: the banner, its
reviewer-controlled semantics, and the "detect a change made by someone else" behaviour already live
in `external-change-notification`, and splitting a second trigger for the same banner into its own
spec would scatter one behaviour across two documents.

### Modified Capabilities

- `external-change-notification`: widens from "the state file changed" to "the review changed
  underneath the reviewer". Adds a requirement to detect new commits on the source branch and raise
  the same banner; adds a requirement that a watch suspends while its alert is raised and re-baselines
  on resume; modifies the banner requirement so its message identifies which source (or sources)
  triggered it. The capability's Purpose paragraph is widened to match at sync time.
- `working-tree-status`: modifies the page-wide banner requirement to drop the file counts, and the
  live-banner-update requirement so that, once the banner is raised, further dirty states are not
  re-reported while it remains up; the return to clean is still detected and clears it. Both existing
  requirements name the counts, so both need the delta.

## Impact

- `backend/watcher.go` — new `watchSourceBranch` poller; the three interval constants collapse to one
  5s constant; each poller gains a suspend gate.
- `backend/gitquery.go` — new `CommitSHA` query.
- `backend/main.go` — start the new poller in `startup`, next to the other two; hold the per-watcher
  suspend flags; expose `SuspendWatch`/`ResumeWatch` over the bridge.
- `backend/watcher_test.go`, `backend/gitquery_test.go` — coverage for the new poller and query.
- `ts/core/` — new pure banner-text helper for the refresh banner, plus its unit test;
  `workingTreeBannerText` loses its counts (its `WorkingTreeStatus` input and null-for-clean contract
  are unchanged, so `main.ts`'s call site is untouched).
- `ts/main.ts` — subscribe to `commits:changed`, track which sources are pending, render the banner
  text, and clear the pending set on refresh or dismiss.
- `backend/assets/index.html` — the banner's fixed sentence becomes a text span the frontend fills.
- No new dependencies. No change to the refresh path itself: `performFullRefresh` already recomputes
  the diff, re-anchors comments, and evicts stale file marks, which is exactly what a new commit needs.
