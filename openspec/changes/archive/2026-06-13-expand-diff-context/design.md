## Context

The backend obtains a review's diff once at startup via `git diff base...head` (`backend/git.go:42`), parses it with `ParseDiff` into `[]DiffFile` (`backend/diff_parser.go:42`), and serves that structure to the frontend through the Wails-bound `App.GetDiffFiles` (`backend/main.go:132`). The frontend mirrors the structs and renders hunks in `renderDiff` (`frontend/web.go:423`), drawing each line via `createDiffLine`, which sets `data-num` gutter attributes and wires the new-line gutter to the comment modal.

The diff carries only `git`'s default few lines of context per hunk. The full file content is discarded after parsing, so the lines needed to expand context are not in memory. The review model already holds `SourceBranch` and `TargetBranch`, so the revisions to read from git are known.

## Goals / Non-Goals

**Goals:**
- Let a reviewer reveal unchanged lines around any hunk, in fixed steps, without leaving the tool.
- Keep the existing diff JSON shape and bound methods unchanged; add capability additively.
- Keep line-number and comment-anchoring behaviour correct on revealed lines.

**Non-Goals:**
- Showing removed-side (base-revision) full content as a separate column. Expansion reveals unchanged context, which is identical on both sides; the new revision is the single source for revealed lines.
- Persisting expansion state across reloads or into the saved review state.
- Syntax highlighting of revealed lines (tracked as a separate TODO item).
- Re-running `git diff` with a larger `-U` context. We splice ranges instead, so the existing parsed hunks and their comment anchors stay intact.
- Rendering any visual diff for binary files (image diffs, hex views). Binary files get a plain placeholder, nothing more.

## Decisions

### Read full file content with `git show <rev>:<path>`, lazily

Add a git helper that runs `git -C <repo> show <rev>:<path>` and returns the file body. This reaches any committed revision without a working-tree checkout and reuses the existing `os/exec` pattern in `git.go`. The backend caches each `(rev, path)` body in memory on first request so repeated expansions of the same file do not re-shell-out.

*Alternatives considered.* (a) Re-run `git diff -U<large>` and re-parse — rejected: it rebuilds hunks and would disturb existing comment anchoring and require diff-merge logic. (b) Read the working-tree file from disk — rejected: the head revision is a branch, not necessarily the working tree, so disk content can diverge from what is under review.

### Serve context as a line range keyed by new-file line number

Add a bound method, roughly `GetFileLines(path string, startNew, endNew int) (string, error)`, returning the requested inclusive range of new-file lines as JSON `DiffLine` values with `Type=context` and both `OldLineNo`/`NewLineNo` populated. The new revision's content is indexed 1-based by new-file line number; the matching old-file line number for an unchanged region is derived from the bounding hunk's `OldStart`/`NewStart` offset (old = new + (hunkOldStart − hunkNewStart)), which is constant across a contiguous unchanged run.

*Alternative considered.* Returning raw strings and letting the frontend assign numbers — rejected: the old/new offset is a backend concern already implied by the hunk metadata, and returning typed `DiffLine`s lets the frontend reuse `createDiffLine` unchanged.

### Compute gaps and render affordances in the frontend

`renderDiff` already iterates hunks in order. Before the first hunk, between consecutive hunks, and after the last hunk it computes the hidden new-line range from adjacent hunk `NewStart`/`NewLines` (and, for the trailing gap, the file's total line count, which the backend can report alongside the range request or via a small `GetFileLineCount`). Each non-empty gap renders an affordance row. Activating it requests the next step (default 20) from the backend, builds `DiffLine` elements with the existing `createDiffLine`, and splices them into the gap, then recomputes whether the affordance should remain or be removed (gap exhausted) and whether two hunks have become contiguous.

### Single fixed-step interaction

Per the proposal, expansion is a fixed step (default 20) at each boundary, with the step automatically shrinking to the remaining gap when fewer than 20 lines are left (which also serves the small-gap "expand all" case). This is the minimal interaction that satisfies the requirement; a separate always-visible "expand entire gap" control is deferred.

### Detect binary files from the diff marker, guard every body path

`git diff` emits a `diff --git` header for a changed binary file but no hunks — only a `Binary files a/… and b/… differ` line. `ParseDiff` already creates a `DiffFile` with empty `Hunks` for such a section and ignores the marker line. The change adds a `Binary bool` field to `DiffFile` and sets it when the parser sees `Binary files ` while a file section is open. The frontend short-circuits in `renderDiff`: a binary `DiffFile` renders a single placeholder element and returns before any hunk, affordance, or `git show` work. This keeps binary handling a property of the data, not a heuristic re-derived at render time, and guarantees no expansion path ever requests a binary blob.

*Alternative considered.* Probing each file with `git diff --numstat` (where binary files show `-` `-`) — rejected: it adds a git call per file for information the diff already carries inline, and the marker is the same signal `git` itself uses.

### Open files with a backend `xdg-open` call, not a webkit API

The Wails runtime's `BrowserOpenURL` opens a URL in a browser, which is the wrong semantics for "open this file in its preferred application". Instead the change adds a bound `App` method that shells out to `xdg-open <repoPath>/<path>` via `os/exec`, reusing the established git-call pattern in `git.go`/`main.go`. The frontend renders a "browse" link in the filename line that calls this method. The target is the working-tree path (the reviewer's actual file on disk), per the decision to favour the live file over a throwaway temp checkout of the head revision.

*Alternatives considered.* (a) Runtime `BrowserOpenURL` with a `file://` URL — rejected: it routes through the browser-open handler rather than the desktop file association, and behaviour for local files is inconsistent. (b) `git show head:<path>` to a temp file then open — rejected by the working-tree decision; it also litters temp files and shows the reviewer a meaningless path. The working-tree path can diverge from the head revision under review, which is an accepted trade-off for opening the real, editable file.

## Risks / Trade-offs

- **Revision content diverges from the parsed diff** (e.g. file renamed, CRLF differences) → derive revealed lines from the same head revision the diff was generated against, and validate that a revealed context line's text matches the hunk's adjacent context line where they overlap; on mismatch, fail the expansion for that gap rather than show wrong lines.
- **Large files re-fetched repeatedly** → cache file bodies per `(rev, path)` in the backend for the session.
- **Comment anchoring on revealed lines** → comments are keyed by new-file line number and looked up per line in `getCommentsForLine`; reusing that lookup when rendering revealed lines keeps anchoring correct with no new state.
- **Binary or deleted files** → `git show` on a deleted-at-head path or binary blob is handled by returning an error; the frontend simply omits affordances when a range request fails.
- **Off-by-one in old/new line mapping** → covered by unit tests on the range/offset computation in the backend, the highest-value place to test since the frontend is GopherJS and harder to unit test.
- **`xdg-open` unavailable or non-Linux host** → the bound method returns the exec error; the frontend reports it without changing review state. The feature is Linux-oriented by the `xdg-open` choice and is not gated for other platforms in this change.
- **Binary detection misses a file `git` omits entirely** → only files with a diff section are classified; a file `git` does not report as changed never reaches the parser, which is consistent with the rest of the tool only knowing the diff.
