## Why

A comment can only be attached to a diff line. Binary files render an explanatory
placeholder with no lines, so today they cannot be commented on at all — the case
that prompted this change. More generally there is no way to leave a remark about a
whole file (e.g. "this file should be split") rather than one line of it.

## What Changes

- Add a **per-file top-level comment**: feedback attached to a file but to no line.
  It is stored as an anchorless root comment in that file's `FileDiff.Comments`,
  reusing the exact shape already used by review-level "overall" comments. No comment
  schema change is required.
- Add a backend `AddFileComment(filePath, content)` mutation (a thin App method over
  the existing `FileDiff.AddComment(content, 0, author)`), returning the standard
  incremental `CommentMutationResult` like `AddReviewComment`.
- Render a **top-level section at the top of the single-file view**, after the file
  header and before any code: an "Add comment" button that opens the familiar comment
  modal, followed by the file's existing top-level comment threads.
- Fix the top-of-file ordering: **top-level comments first, then the untethered
  (outdated) comments block, then the code hunks.**
- Make the affordance work for **binary files** — the motivating case. The "Add
  comment" button and any top-level threads appear together with the "binary file,
  cannot diff" placeholder, and adding a comment does not fetch or render a diff body.
- Show a file's top-level comments within its section in the **review overview**, so a
  binary file whose only feedback is a top-level comment still appears there.
- Add a file-comment mode to the add-comment modal and an `addFileComment` client
  wrapper so saving routes to the new mutation.

The read-only CLI already iterates each file's comments generically and will surface a
top-level comment with line `0` (as it does review-level comments), so it needs no
change.

## Capabilities

### New Capabilities
- `top-level-file-comments`: a reviewer can leave a comment scoped to a whole file
  (no line), including on binary files; its data shape, the top-of-file "Add comment"
  affordance, its incremental save, and its appearance in the single-file view and the
  overview.

### Modified Capabilities
- `comment-reanchoring`: the requirement that outdated comments render untethered at
  the top of the file view is refined so that they render **below** any top-level
  file comments (still above the code), fixing the top-of-file order to top-level
  comments → untethered comments → code.

## Impact

- Backend: `backend/main.go` (new `AddFileComment` App method; no model change —
  `FileDiff.AddComment(_, 0, _)` already yields an anchorless comment).
- Frontend core: `ts/core/comments.ts` (new pure `topLevelComments` predicate + tests).
- Frontend render: `ts/render/hunks.ts` (top-of-file section + ordering, binary path),
  `ts/render/comments.ts` (top-level thread block), `ts/render/modals.ts` (file-comment
  mode), `ts/render/overview.ts` (per-file top-level comments), `ts/client.ts`
  (`addFileComment`), and Wails bindings.
- No change to the comment on-disk schema, to the re-anchoring algorithm, or to the CLI.
