## 1. Verify rename detection and parsing

- [x] 1.1 Confirm `backend/git.go` `GetDiff` passes `-M` so a pure rename is reported as a rename regardless of `diff.renames`.
- [x] 1.2 Confirm `backend/diff_parser.go` flags a renamed file (`DiffFile.Renamed`) and records its pre-rename path (`DiffFile.OldPath`) from the `rename from <path>` marker, mirroring binary detection.
- [x] 1.3 Confirm `backend/main.go` `GetDiffFiles` metadata carries `Renamed` and `OldPath` to the frontend.

## 2. Verify the placeholder rendering

- [x] 2.1 Confirm `ts/core/types.ts` `DiffFile` and `DiffFileMeta` carry required `Renamed` and `OldPath` fields.
- [x] 2.2 Confirm `ts/render/load.ts` and `ts/render/state.ts` carry `Renamed`/`OldPath` through state and the lazy fetch.
- [x] 2.3 Confirm `ts/render/hunks.ts` renders the `.rename-placeholder` with "renamed from `<old>`, no changes" for a pure rename (`Renamed && Hunks.length === 0`), and falls back to "renamed, no changes" when `OldPath` is empty.
- [x] 2.4 Confirm a rename with edits keeps its hunks and shows no placeholder.
- [x] 2.5 Confirm `backend/assets/style.css` `.rename-placeholder` shares `.binary-placeholder` styling.

## 3. Verify tests cover the requirement

- [x] 3.1 Confirm `backend/diff_parser_test.go` covers a pure rename (`TestParseDiffPureRename`) and a rename with edits (`TestParseDiffRenameWithEdits`), and they pass: `go test ./... -run 'TestParseDiffPureRename|TestParseDiffRenameWithEdits'`.
- [x] 3.2 Confirm `backend/git_test.go` proves `-M` forces rename detection with `diff.renames=false` (`TestGetDiffPureRenameWithRenamesDisabled`) and passes.
- [x] 3.3 Confirm `ts/render/render_dom_test.ts` asserts the placeholder text and the rename-with-edits path, and the suite passes: `deno task test`.

## 4. Validate the change

- [x] 4.1 Run `openspec validate rename-placeholder` and resolve any reported issues.
