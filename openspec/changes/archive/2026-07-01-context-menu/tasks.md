## 1. Enable the default context menu

- [x] 1.1 Set `EnableDefaultContextMenu: true` on the `options.App` literal passed to `wails.Run` in `backend/main.go`.

## 2. Confirm the build

- [x] 2.1 Build the project and confirm it compiles with the option set (`go build ./...` after `deno task bundle`; `go vet ./...` clean).
- [x] 2.2 Run the existing test suites to confirm no regression (Go: ok; frontend: 98 passed, 0 failed).

## 3. Verify behaviour in the released binary

- [x] 3.1 Run the released binary, select diff lines, right-click, and confirm a context menu with a copy action appears.
- [x] 3.2 Confirm a copied selection contains the code text only, without the gutter line numbers.

## 4. Validate the change

- [x] 4.1 Run `openspec validate context-menu` and resolve any reported issues (valid).
