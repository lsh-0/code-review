## 1. Verify implementation matches the tightened requirement

- [x] 1.1 Confirm `backend/diff_parser.go` detects the `Binary files ... differ` marker and sets `DiffFile.Binary`.
- [x] 1.2 Confirm `ts/render/hunks.ts` renders the `.binary-placeholder` element with the exact text "binary file, cannot diff" and renders no hunks or expansion affordances for a binary file.
- [x] 1.3 Confirm no blob content is fetched for a binary file (the placeholder branch returns before any blob request).

## 2. Verify tests cover the requirement

- [x] 2.1 Confirm `backend` has a test covering binary detection (`TestParseDiffBinaryFile`) and that it passes: `go test ./... -run TestParseDiffBinaryFile`.
- [x] 2.2 Confirm `ts/render/render_dom_test.ts` asserts the exact placeholder string and passes: `deno test ts/render/render_dom_test.ts`.

## 3. Validate the change

- [x] 3.1 Run `openspec validate explanatory-binary-placeholder` and resolve any reported issues.
