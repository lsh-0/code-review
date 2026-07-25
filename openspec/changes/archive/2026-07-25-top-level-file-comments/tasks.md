## 1. Backend mutation

- [x] 1.1 Add `App.AddFileComment(filePath, content string) (string, error)` in `backend/main.go`, mirroring `AddReviewComment`: resolve the `FileDiff`, call `FileDiff.AddComment(content, 0, a.userName)`, persist, and return `mutationResult(filePath, 0)`. Return an error when the file has no diff.
- [x] 1.2 Regenerate the Wails bindings so the frontend can call `AddFileComment`.
- [x] 1.3 Add a Go test that adding a file comment appends an anchorless root comment (empty `Anchors`, `CurrentLineNumber()==0`, `IsOutdated()==false`) and that `GetCommentedFiles` includes a binary file whose only comment is top-level.

## 2. Frontend core

- [x] 2.1 Add pure `topLevelComments(comments)` to `ts/core/comments.ts`: root comments that are anchorless and not outdated (`!parent_id && !isOutdated(c) && currentLineNumber(c) === 0`), with a doc comment noting it is the third bucket beside `getCommentsForLine` and `outdatedComments`.
- [x] 2.2 Add tests in `ts/core/comments_test.ts`: a top-level comment is returned; a line comment, an outdated comment, and a reply are excluded; the buckets are disjoint.

## 3. Client + modal

- [x] 3.1 Add `addFileComment(filePath, content)` to `ts/client.ts` calling the new `AddFileComment` binding and returning the `CommentMutationResult`.
- [x] 3.2 Add `fileCommentMode` to the modal state and `showFileCommentModal(filePath)` in `ts/render/modals.ts` (analogous to `showReviewCommentModal`); route `saveComment` to `api.addFileComment(state.modal.file, content)` when in that mode.

## 4. Render — single-file view

- [x] 4.1 Add `topLevelCommentsBlock(filePath, comments, actions)` to `ts/render/comments.ts` mirroring `outdatedCommentsBlock`: an "Add comment" button (opens `showFileCommentModal`) plus a thread for each top-level comment, carrying a `data-toplevel` marker; return the block even when there are no top-level comments (so the button always shows), or a variant that always shows the button.
- [x] 4.2 In `renderFileHunks` (`ts/render/hunks.ts`), append the top-level block first, then the untethered outdated block, then the code — enforcing order top-level → untethered → code. Emit the top-level block for binary files too, before the placeholder early-return, without fetching blob/diff content.
- [x] 4.3 Add `updateTopLevelBlock` in `ts/render/mutate.ts` (mirroring the outdated-block rebuild): insert the block at the top of the file region when absent, replace when present, following an `AddFileComment`/status/delete mutation.
- [x] 4.4 Style the top-level section in `backend/assets/style.css` consistent with the existing outdated/overview comment blocks.

## 5. Render — overview

- [x] 5.1 In `ts/render/overview.ts`, render each file's top-level comment threads within that file's overview section (via `topLevelComments` + `createCommentThread`), so a file — including a binary file — whose only feedback is top-level shows it. Do not add the top-of-file "Add comment" button to the overview.

## 6. Render tests

- [x] 6.1 Add DOM tests in `ts/render/render_dom_test.ts`: the single-file view renders the "Add comment" control and top-level threads at the top; ordering is top-level → untethered → code when both exist; a binary file renders the control and any top-level threads with no diff body; the overview shows a binary file's top-level comment.

## 7. Verify

- [x] 7.1 Run `openspec validate top-level-file-comments`, the Go tests, and the Deno tests; confirm all pass.
