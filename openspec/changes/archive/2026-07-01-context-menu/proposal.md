## Why

Right-clicking a selection in the diff offers nothing in a released build. The
motivating case is copying selected diff lines: a reviewer who selects text and
right-clicks expects at least a copy action, and gets an empty gesture instead.

This lack of basic functionality is an accessibility failure. 

The cause is a Wails default, not missing application code. Wails v2 disables the
webview's built-in context menu in production builds; it is present only in
development (`wails dev`) and debug builds. That is exactly the asymmetry the user
observed — a native-looking context menu under `wails dev`, none in the installed
binary. The menu therefore does not need to be built; it needs to be turned on for
production.

## What Changes

- The released GUI SHALL present a context menu on right-click, with at least the
  copy action over a text selection, matching the menu already available in
  development.
- This is enabled by setting `EnableDefaultContextMenu: true` on the Wails
  `options.App` passed to `wails.Run` in `backend/main.go`. No custom menu is built;
  the webview's own menu is used, so the menu is native-looking and platform-portable
  (WebKitGTK on Linux, WebKit on macOS).
- With this option set, Wails shows the menu only in text contexts (where
  cut/copy/paste apply) by default. The diff body is selectable text, so the copy
  action appears on a diff selection. No CSS override (`--default-contextmenu`) is
  added, so the menu is scoped to text rather than offered over the whole window.

## Capabilities

### New Capabilities

- `context-menu`: the released GUI exposes the webview's default context menu so a
  right-click over a text selection offers cut/copy/paste.

### Modified Capabilities

None.

## Impact

- `backend/main.go` — the `options.App` literal gains `EnableDefaultContextMenu: true`.
- No frontend change. The diff already renders selectable text (line numbers are
  drawn via CSS `::before` so they are excluded from the selection), so a copied
  selection is the code only, with no change needed.
- No new dependency, no new bound method, no growth in binary size beyond the
  unchanged Wails runtime. The portability goal is preserved: the menu is the
  webview's own, not a bespoke DOM widget.
