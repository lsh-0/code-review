## 1. Backend: read full file content from git

- [x] 1.1 Add a `git.go` helper that runs `git -C <repo> show <rev>:<path>` and returns the file body (and a not-found error when the path/rev is absent)
- [x] 1.2 Add a unit test in `git_test.go` covering an existing file at a branch revision and a missing path, reusing `setupTestRepo`
- [x] 1.3 Add an in-memory per-`(rev, path)` cache for file bodies so repeated expansions do not re-shell-out

## 2. Backend: serve context line ranges

- [x] 2.1 Add a function that, given a file body and an inclusive new-line range, returns `[]DiffLine` of type context with `OldLineNo`/`NewLineNo` populated from a supplied old/new offset
- [x] 2.2 Unit-test the range-to-`DiffLine` mapping in `diff_parser_test.go` style, including the old/new offset and a clamped range at end-of-file (given/expected/actual naming)
- [x] 2.3 Add a Wails-bound `App` method returning the requested range as JSON for a path at the head revision, plus the file's total line count for trailing-gap computation
- [x] 2.4 Wire the method to the file-body cache and the source-branch revision held on the review model; cover the success and missing-path paths in `main_test.go`

## 3. Binary files: detect and refuse to render

- [x] 3.1 Add a `Binary bool` field to `DiffFile` (backend and the frontend mirror struct) and set it in `ParseDiff` when a `Binary files ... differ` line appears within an open file section
- [x] 3.2 Unit-test `ParseDiff` in `diff_parser_test.go` on a diff containing both a text file and a binary file, asserting the binary file has `Binary == true` and no hunks (given/expected/actual naming)
- [x] 3.3 In `renderDiff`, short-circuit a binary `DiffFile` to a plain placeholder element and return before any hunk, affordance, or blob-fetch work
- [x] 3.4 Add CSS in `assets/style.css` for the binary placeholder

## 4. Browse: open a file in the preferred application

- [x] 4.1 Add a `git.go`/backend helper and a Wails-bound `App` method that runs `xdg-open <repoPath>/<path>` and returns the exec error on failure
- [x] 4.2 Unit-test the path composition and error propagation for a missing opener/path in `main_test.go`
- [x] 4.3 Render a "browse" link in the filename line that calls the method; report failures without changing review state
- [x] 4.4 Add CSS in `assets/style.css` for the browse link

## 5. Frontend: gap detection and affordances

- [x] 5.1 In `renderDiff`, compute the hidden new-line gap before the first hunk, between adjacent hunks, and after the last hunk
- [x] 5.2 Render an expansion affordance row for each non-empty gap, with up/down direction as appropriate
- [x] 5.3 Add CSS in `assets/style.css` for the affordance rows, matching the existing hunk-header styling

## 6. Frontend: expansion behaviour

- [x] 6.1 On affordance activation, request the next step (default 20, clamped to the remaining gap) from the backend
- [x] 6.2 Build revealed lines with the existing `createDiffLine` and splice them into the gap in correct file order
- [x] 6.3 After splicing, re-evaluate the gap: remove the affordance when exhausted and join the two hunks visually when a between-hunks gap is fully revealed
- [x] 6.4 Ensure revealed lines render their comment threads via the existing `getCommentsForLine` lookup and keep the clickable new-line gutter

## 7. Verification

- [x] 7.1 Run `go test ./...` and confirm backend tests pass
- [x] 7.2 Build and manually verify expansion above, between, and below hunks, including a gap smaller than one step and a comment on a revealed line
- [ ] 7.3 Manually verify a binary file lists in the nav and shows the placeholder (no diff body) when selected
- [x] 7.4 Manually verify the browse link opens a changed file in the preferred application
- [x] 7.5 Remove the corresponding "Expand diff context" entry from `TODO.md`
