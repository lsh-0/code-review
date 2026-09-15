## Context

Comments in this tool anchor to a diff line: a `Comment` carries an ordered `Anchors`
history keyed by blob SHA (see `comment-reanchoring`). Two comment kinds already break
that pattern by carrying **no** anchor: replies (a `parent_id`, no line) and review-level
"overall" comments in `Review.Comments`. Both are produced by `NewComment(content, 0,
author)`, whose `lineNumber == 0` branch leaves `Anchors` empty.

Binary files render a placeholder ("binary file, cannot diff") with no lines, so they have
nothing to click to attach a comment — the reviewer cannot comment on them at all. There
is also no whole-file remark for text files.

The frontend is a pure core (`ts/core`), a typed RPC client (`ts/client.ts`), and a render
layer (`ts/render`). The single-file view is built by `renderFileHunks` in
`ts/render/hunks.ts`; it already renders the untethered outdated-comments block at the top
via `outdatedCommentsBlock` (`ts/render/comments.ts`). The overview is built by
`renderOverview` (`ts/render/overview.ts`), which has a review-level "General review
comments" section with its own "Add comment" button routed to `AddReviewComment`.

## Goals / Non-Goals

**Goals:**
- Let a reviewer leave a comment scoped to a whole file, with no line, including on binary files.
- Reuse the existing anchorless-comment machinery — no comment schema or re-anchoring change.
- Place the affordance and threads at the top of the file, ordered top-level → untethered → code.
- Surface per-file top-level comments in the overview.

**Non-Goals:**
- No change to the re-anchoring algorithm, the on-disk comment schema, or the CLI.
- No new comment status semantics; top-level comments use the existing active/resolved/ignored flow.
- Not adding top-level comments to files that are unchanged / absent from the diff.

## Decisions

### Represent a top-level file comment as an anchorless comment in `FileDiff.Comments`

A top-level file comment is a root `Comment` in the file's `Comments` slice with an empty
`Anchors`. This is exactly what `FileDiff.AddComment(content, 0, author)` already produces
(it calls `NewComment`, whose `lineNumber == 0` path appends no anchor). Consequences fall
out for free:
- `CurrentLineNumber() == 0` and `IsOutdated() == false`.
- `getCommentsForLine(comments, n)` never matches it (it needs `currentLineNumber === n`, and real lines are `> 0`).
- `outdatedComments(comments)` never matches it (not outdated).
- `fileCommentStatus` already counts it as a root, so file status aggregates correctly.
- `GetCommentedFiles` already includes any file whose `Comments` is non-empty, so a binary file with only a top-level comment appears in the overview with no backend change.

*Alternative considered:* a new `Comment` field (e.g. `Scope: "file"`). Rejected — it
duplicates the information already carried by "root + no anchor", forces a schema/migration
change, and adds a second source of truth the reanchoring code would have to respect.

### Backend: one thin App method `AddFileComment(filePath, content)`

Mirror `AddReviewComment`: resolve the `FileDiff`, call `FileDiff.AddComment(content, 0,
a.userName)`, persist, and return `mutationResult(filePath, 0)` (the standard
`CommentMutationResult`). No model method is added. The Wails binding is regenerated so the
client can call it.

### Frontend core: a `topLevelComments` predicate

Add a pure `topLevelComments(comments)` to `ts/core/comments.ts` returning root comments
that are anchorless — `!parent_id && !isOutdated(c) && currentLineNumber(c) === 0` — the
third bucket beside `getCommentsForLine` and `outdatedComments`. Unit-tested in
`comments_test.ts` with no DOM.

### Render: a top-of-file section, ordered above the untethered block

In `renderFileHunks`, before the existing outdated block and before the (binary or text)
body, append a top-of-file section: an "Add comment" button (opens the comment modal in a
new file-comment mode) followed by a thread block built from `topLevelComments(...)`. The
existing outdated block is appended after it, then the hunks — giving the required order
top-level → untethered → code. For binary files this section is emitted **before** the
early-return that writes the placeholder, so the button and threads appear with the
placeholder and no blob/diff body is fetched. A new `topLevelCommentsBlock(filePath,
comments, actions)` in `ts/render/comments.ts` mirrors `outdatedCommentsBlock` and carries
a `data-toplevel` marker so `ts/render/mutate.ts` can replace it incrementally after a
mutation, the same way the outdated block is rebuilt.

### Modal + client: a file-comment mode

Add `showFileCommentModal(filePath)` setting a `fileCommentMode` flag (analogous to
`reviewCommentMode`) and, in `saveComment`, route that mode to `api.addFileComment(state
.modal.file, content)`. Add `addFileComment(filePath, content)` to `ts/client.ts` calling
the new binding.

### Overview: render each file's top-level comments in its section

`renderOverview` already fetches and lays out each commented file's section. Include the
file's top-level comment threads in that section (via the shared `topLevelCommentsBlock` or
`createCommentThread` over `topLevelComments`), so a file — including a binary file — whose
only feedback is top-level shows it in the overview. The overview does not show the
top-of-file "Add comment" button (feedback is authored in the single-file view; the
overview's own add-control stays review-level).

## Risks / Trade-offs

- **A line comment that goes adrift also has line 0** → It is distinguished by being
  *outdated* (adrift anchor), so `topLevelComments` excludes it (it requires `!isOutdated`);
  such comments stay in the untethered bucket. The two buckets are disjoint.
- **Binary early-return path** → The top-level section must be emitted before the binary
  placeholder returns; a regression there would drop the affordance on exactly the
  motivating case, so it is covered by a render (DOM) test asserting the button and any
  threads render for a binary file with no diff body fetched.
- **Incremental update coverage** → Adding the first top-level comment must insert the block
  even when none existed; `updateTopLevelBlock` in `mutate.ts` follows the existing
  outdated-block pattern (insert at top when absent, replace when present, remove when
  empty).

## Open Questions

- Whether the top-of-file "Add comment" control should read "Add comment" or something more
  specific (e.g. "Comment on file"); left to the reviewer's preference at the review gate.
  Default: reuse the familiar "Add comment" label already used by the overview control.
