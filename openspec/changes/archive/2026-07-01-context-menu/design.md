## Context

Wails v2 disables the webview's built-in context menu in production builds. It is
available in `wails dev` and in `wails build -debug`/`-devtools`, which is why the
menu appears under dev mode but not in the installed binary. The behaviour is gated
by a single option on `options.App`:

```
// EnableDefaultContextMenu enables the browser's default context-menu in production
EnableDefaultContextMenu bool
```

confirmed present in the project's pinned Wails version (`v2.12.0`) via
`go doc github.com/wailsapp/wails/v2/pkg/options.App`.

## Goals / Non-Goals

- Goal: a right-click context menu in the released GUI with at least a copy action
  over a selection.
- Goal: keep it native-looking and portable, matching what the user already saw in
  dev mode.
- Non-goal: a bespoke, application-drawn context menu. That would add a DOM widget,
  selection-coordinate handling, and per-platform styling for no benefit over the
  webview's own menu, and would work against the project's portability and
  small-surface goals.
- Non-goal: review-specific menu actions (e.g. "add comment here" on right-click).
  The card asks for copy as the motivating case; richer actions are out of scope and
  can be a later TODO.

## Decisions

### Use the webview's default menu, not a custom one

`EnableDefaultContextMenu: true` re-enables the webview's own menu in production.
This is one line, has no runtime cost, and yields a native menu on both WebKitGTK
(Linux) and WebKit (macOS). A custom menu would be more code and less native for no
gain against the motivating case.

### Leave the menu scoped to text contexts

With the option enabled, Wails shows the menu only where cut/copy/paste apply (text
selections and inputs) unless a `--default-contextmenu` CSS rule widens it. The
motivating case is copying a diff selection, which is a text context, so the default
scoping already covers it. Not adding the CSS override keeps the menu from appearing
over non-text regions (the file list, the diff gutter), which would offer no useful
action.

### No frontend change is required for copy correctness

Diff line numbers are rendered with CSS (`.line-number::before { content: attr(...) }`),
so they are not part of the selectable text. A selection over diff rows already
yields the code text alone, so the copy action copies what the reviewer expects with
no change to the render layer.

## Risks / Trade-offs

- The webview's menu may also include entries such as "save page" or "reload" on some
  platforms. This is the standard tradeoff for the native menu and is accepted as
  consistent with the dev-mode menu the user already found acceptable.
- The Wails note that this option does not work with Vite v5+ does not apply: this
  project builds its frontend with the Deno bundler, not Vite.

## Verification

This is a webview-runtime behaviour, set by a build option, and is not exercisable by
the Go or TypeScript unit tests (which run headless, without a webview). It is
verified by building and running the released binary and confirming a right-click
over a diff selection offers copy. The build must succeed with the option set
(`./manage.sh build`).
