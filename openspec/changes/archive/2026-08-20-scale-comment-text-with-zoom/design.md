## Context

Zoom is implemented once, in `applyZoom` (`ts/main.ts:273-276`): clamp the factor
to 0.5–3.0 and write it to `--zoom` on `documentElement`. Everything downstream is
CSS. The diff side consumes it through a two-variable pattern in `:root`
(`backend/assets/style.css:51-55`):

```css
--diff-font-base: 0.9rem;
--zoom: 1;
--diff-font-size: calc(var(--diff-font-base) * var(--zoom));
```

`--diff-font-size` is then used by `.diff-line`, `.hunk-header`, `.expand-row` and
the placeholders; `.line-number` opts in separately with
`calc(0.8rem * var(--zoom))`.

The comment side never opted in. `.comment-content` declares no `font-size` at all,
so it inherits `body` (1rem = 20px, since `html` is at 125%); `.comment-status` and
`.comment-author` are 0.75rem, `.comment-actions button` is 0.8rem, and the modal
textarea is 1.05rem. None of them move when `--zoom` does.

Two structural facts from `ts/render/comments.ts` shape the design:

1. **Every thread goes through one container.** `createCommentThread` produces a
   `.comment-thread` div (line 37), and all three contexts route through it —
   line-anchored (`lineCommentThread`, 149), top-level (`topLevelCommentsBlock`,
   84) and outdated (`outdatedCommentItem`, 136). One declaration on
   `.comment-thread` therefore reaches every comment on screen.
2. **The thread is flat.** A reply is a `.comment.comment-reply` placed directly in
   a `.comment-replies` rail (194-200), replies of replies are not rendered
   (`if (!isReply)`, 191), and neither `.comment` nor `.comment-reply` declares a
   `font-size`. There is exactly one font-size boundary between `.comment-thread`
   and any text leaf.

Constraint: `backend/assets/` is `//go:embed`-compiled, so nothing here is
verifiable by refreshing a page — it needs a rebuild and app restart.

## Goals / Non-Goals

**Goals:**

- Comment text answers to the same zoom control as diff code, at the proportions
  it already has.
- Zoom factor 1.0 renders pixel-identical to today. This is a scaling change, not
  a restyling; nobody's default view should shift.
- One place to change the comment scale, mirroring the existing diff pattern, so
  the next person finds the same shape on both sides.

**Non-Goals:**

- Scaling application chrome (header, sidebar, file list, comment-count badge).
  Zoom is for the reading surface; growing the navigation with it would eat the
  window.
- A separate zoom control for comments. One factor, one control — two would be a
  worse tool.
- Persisting the zoom factor across restarts. Out of scope; it is session state
  today and stays so.
- Deleting the dead `.comment-btn` / `.line-actions` rules. Real, but a separate
  cleanup (see Decision 6).
- Any change to `ts/`.

## Decisions

### Decision 1: Scale from the container with `em` leaves, not `var(--zoom)` per rule

Declare the scaled size once on `.comment-thread` and convert the descendants'
fixed `rem` sizes to `em`, so they scale off their ancestor:

```css
.comment-thread { font-size: var(--comment-font-size); }
.comment-status  { font-size: 0.75em; }   /* was 0.75rem */
.comment-author  { font-size: 0.75em; }   /* was 0.75rem */
.comment-actions button { font-size: 0.8em; }  /* was 0.8rem */
```

`.comment-content` needs no rule at all — it inherits the container's size, which
is exactly why it inherits `body` today.

*Alternative considered:* give each rule its own `calc(0.75rem * var(--zoom))`,
matching `.line-number`. Rejected: it repeats the zoom term in five places, and the
proportions between the parts stop being visible in the source — `0.75em` states
"three-quarters of the comment text" directly, whereas
`calc(0.75rem * var(--zoom))` states it only by coincidence of sharing a
multiplier. One container declaration is also the difference between adding a
comment sub-element correctly by default and having to remember to opt it in.

*Why `em` is safe here specifically:* it compounds through nested font-size
declarations, and the fact-finding above establishes there are none — the thread
is flat and the intermediate `.comment`, `.comment-reply`, `.comment-header`,
`.comment-replies` elements declare no font-size. Keep it that way: `em` sizes
belong on leaves. If threads ever nest, this decision needs revisiting, which
Risk 1 records.

### Decision 2: A `--comment-font-base` / `--comment-font-size` pair, base `1rem`

Add to `:root`, directly under the diff pair, so the two sit together:

```css
--comment-font-base: 1rem;
--comment-font-size: calc(var(--comment-font-base) * var(--zoom));
```

Base `1rem` is chosen because it is what `.comment-content` inherits from `body`
today — that is what makes zoom 1.0 pixel-identical. It is deliberately not folded
into `--diff-font-base` (0.9rem): comments are sans-serif prose and code is
monospace, and the two want different base sizes even though they want the same
scale factor. Keeping separate bases with a shared `--zoom` multiplier expresses
exactly that.

### Decision 3: The card's chrome stays fixed; only text scales

`.comment-thread`'s `padding: 0.75rem` and `.comment`'s padding/margins stay in
`rem` and do not grow with zoom.

This is not laziness — the existing comment on that rule (`style.css:909-912`)
records that the 0.75rem inset is a shared indent step aligning the hunk edge, the
comment card edge and the reply rail. Converting it to `em` would scale the comment
card's inset while the hunk's stayed put, and the alignment it exists to maintain
would drift apart as the user zoomed. Fixed padding around large text is the lesser
cost; it reads slightly tighter at 3.0 but stays aligned.

### Decision 4: The compose modal scales its text *and* its width

The textarea takes `calc(1.05rem * var(--zoom))` — it is one rule outside any
`.comment-thread`, so it uses the zoom term directly rather than `em`.

Scaling the text alone is not enough. `.modal-content` is `width: 90%;
max-width: 600px`, so at zoom 3.0 the text is ~63px in a 600px box — under ten
characters a line. So `max-width` becomes `calc(600px * var(--zoom))`. The existing
`width: 90%` is what keeps it on screen: the modal takes the smaller of the two, so
it grows with zoom until it hits 90% of the window and then stops. That gives the
"stays inside the window" scenario for free, with no media query and no JS.

*Alternative considered:* leave the modal unscaled, on the grounds that it is
chrome. Rejected: it is where comment text is *written*, and writing at a different
size from reading is the same mismatch this change exists to remove.

*Revised at the second review gate:* the modal scales **as a whole** — title
(`.modal-content h3`), textarea, and action buttons (`.modal-actions button`).
The first pass scaled only the textarea and the width, leaving the title and
buttons fixed on the reading that they were frame chrome. That was wrong in
practice: all three modals in `index.html` are comment modals (add / edit /
reply), so every part of them is a comment surface, and a box that grew in parts
read as broken rather than zoomed. The user asked for the title specifically; the
buttons had already been corrected during the verify pass.

The file-content header (`#current-file-name`) is the boundary case on the other
side, and stays fixed: it labels the view, not a comment. Confirmed by the user.

### Decision 5: Outdated captured hunks keep the code scale — no change

The original report listed `.outdated-line` as a fixed-size comment surface. It is
not: the renderer emits it as `["diff-line", "outdated-line"]`
(`ts/render/comments.ts:130`), so `.diff-line`'s `font-size: var(--diff-font-size)`
already applies. It is code and it already scales at the code scale, which is
right. Deliberately untouched — a `.outdated-line` font-size rule added here would
override `.diff-line` and *break* working behaviour.

### Decision 6: Dead rules are left exactly as they are

`.comment-btn` (858) and `.line-actions` (847) have no emitter in `ts/` or
`index.html`; the Deno frontend starts a line comment from `.line-number.clickable`.
They are dead. They are not scaled (pointless) and not deleted (out of scope) —
touching them here would put an unrelated cleanup inside a zoom change and muddy
the diff. Flagged in the proposal so it can be raised as its own item.

### Decision 7: The chrome boundary

Scaled: comment threads and their contents, the add-comment controls beside them,
the compose modal. Not scaled: header, sidebar, file list, `.comment-count` badge,
toolbar buttons.

The line is "text under review, and the controls that act directly on it" versus
"navigation and application frame". The `.comment-count` badge is the interesting
case — it is about comments, so it looks like it belongs — but it lives in the
sidebar file list, and scaling one badge inside otherwise fixed-size chrome would
misalign the file list rows for no benefit. It stays fixed.

## Risks / Trade-offs

- **`em` compounds if comment threads ever nest** (a nested `.comment-thread`, or a
  `font-size` added to `.comment`) → the flatness this relies on is asserted by
  `ts/render/comments.ts:191` (replies of replies are not rendered). Mitigation:
  keep `em` on leaf elements only, and note the assumption in the CSS comment on
  `.comment-thread` so the next editor sees the constraint at the point of change.
- **Fixed padding around 3.0-scale text reads tight** → accepted deliberately in
  Decision 3 to preserve the hunk/comment/reply edge alignment. Reversible in
  isolation later if it proves annoying in use.
- **Long unbroken comment text could overflow at high zoom** → `.comment-content`
  is already `white-space: pre-wrap`, which wraps; no new overflow mode is
  introduced. Worth an eye during verification at 3.0.
- **Verification requires a rebuild, so a stale bundle can fake a pass or a fail**
  → the change is CSS-only, but `backend/assets` is embedded; confirm against a
  rebuilt binary, not a refresh.
- **Only three sizes are converted to `em`** → if a comment sub-element is added
  later with a `rem` size it will silently not scale. Mitigation is Decision 1
  itself: the container declaration means the *default* (inheriting, like
  `.comment-content`) is now the correct behaviour, so the failure mode requires
  actively opting out.

## Migration Plan

None required. No data, state-file, or API change; the zoom factor is in-memory
session state. Rollback is reverting the CSS. At zoom 1.0 the rendering is
unchanged, so there is nothing for an existing user to adapt to.

## Open Questions

None blocking. One judgement call is flagged for the review gate: Decision 3 keeps
comment card padding fixed to preserve edge alignment with the hunk. If the user
would rather have the card breathe at high zoom and accept the drift, that is a
one-line change to `em` — worth their opinion, not worth blocking on.
