## Why

Ctrl+wheel / Ctrl+= zoom exists so a reviewer can dial the review text to a
comfortable size. It only half works: `applyZoom` sets `--zoom` on the root
element (`ts/main.ts:275`), but only the diff code surfaces consume it. Every
comment font size is fixed, so a reviewer who zooms in to read the code is left
reading the review conversation at the unzoomed size — the smaller of the two
texts stays small precisely when they asked for bigger. Comments are half of what
this tool exists to display; they should answer to the same control as the code.

## What Changes

- Comment text scales with `--zoom`, at the proportions it already has relative
  to the rest of the comment card: body text, author, status badge, and the
  thread's action buttons. This covers all three thread contexts (line-anchored,
  top-level, and outdated) through the shared `.comment-thread` container.
- The "Add comment" affordances that sit alongside comment threads
  (`.top-level-add-comment`, `.overview-add-comment`) scale too, so a control does
  not sit at a fixed size next to the text it acts on.
- The comment compose/edit modal scales: its textarea follows the comment text
  scale, and the modal's `max-width` grows with zoom so a zoomed-in comment is not
  typed into a fixed 600px box a few characters wide. The existing `width: 90%`
  keeps it inside the window.
- Application chrome does **not** scale — the header, sidebar, file list, and the
  file-list `.comment-count` badge keep their current fixed sizes. Zoom governs
  the review reading surface only.
- No TypeScript change: the zoom handlers, the 0.5–3.0 clamp, and Ctrl+0 reset
  (`ts/main.ts:53-55`, `504-521`, `569-578`) are untouched. This is entirely a
  matter of which CSS rules consume `--zoom`.

Two findings from the investigation that narrow the scope from the original
report:

- The outdated comment's captured pseudo-hunk **already scales**. The report
  listed `.outdated-line` as fixed, but the renderer emits it with both classes —
  `["diff-line", "outdated-line"]` (`ts/render/comments.ts:130`) — so it picks up
  `font-size: var(--diff-font-size)` from `.diff-line`. It is code, it scales at
  the code scale, and that is already correct. No change needed.
- `.comment-btn` (`style.css:858`) and its `.line-actions` container (`847`) are
  **dead rules** — nothing in `ts/` or `index.html` emits either class; a line
  comment is started by clicking the line number (`.line-number.clickable`).
  They are left exactly as they are. Deleting dead CSS is a separate cleanup, not
  something to bundle into a zoom change, but it is flagged here so the user can
  raise it as its own item.

## Capabilities

### New Capabilities

- `text-zoom`: the Ctrl+zoom text scale — what a single zoom control governs
  (diff code and comment conversation), what it deliberately leaves alone
  (application chrome), and the range/reset behaviour. No existing spec covers
  zoom at all, so this capability also codifies the pre-existing diff-scaling and
  zoom-control behaviour that the new comment requirement is defined relative to.
  Codified behaviour is marked as such in the spec.

### Modified Capabilities

None. No existing capability's requirements change.

## Impact

- `backend/assets/style.css` — the only file with behavioural change: two new
  `:root` custom properties, a `font-size` on `.comment-thread`, and the comment
  descendants' fixed `rem` sizes converted to `em` so they scale from that one
  declaration. Plus the two add-comment buttons and the modal.
- `backend/assets/` is `//go:embed`-compiled, so verifying this needs a rebuild
  and app restart, not a browser refresh.
- No change to `ts/`, so the existing Deno tests are unaffected; `ts/render`'s
  DOM tests assert classes and structure, not computed font sizes, and none of
  the class names change.
- No API, storage, or state-file impact. Zoom level is in-memory session state
  (`ts/render/state.ts:83`) and is not persisted — unchanged by this.
