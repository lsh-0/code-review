## 1. Define the comment text scale

- [x] 1.1 In `backend/assets/style.css` `:root`, add `--comment-font-base: 1rem`
      and `--comment-font-size: calc(var(--comment-font-base) * var(--zoom))`
      immediately after the existing `--diff-font-size` (line 55), extending the
      block comment there to say that the same `--zoom` drives both the code and
      the comment scale off separate bases (Decision 2).

## 2. Scale comment threads

- [x] 2.1 Add `font-size: var(--comment-font-size)` to `.comment-thread` (906),
      with a comment noting that every thread context routes through this
      container and that descendants size in `em` off it — so the rule must stay
      the only font-size boundary above the text leaves (Decision 1, Risk 1).
- [x] 2.2 Change `.comment-status` (1018) from `0.75rem` to `0.75em`.
- [x] 2.3 Change `.comment-author` (1038) from `0.75rem` to `0.75em`.
- [x] 2.4 Change `.comment-actions button` (1091) from `0.8rem` to `0.8em`.
- [x] 2.5 Confirm `.comment-content` (1043) still declares no `font-size` — it
      must keep inheriting, which is what makes it scale. Add no rule.
- [x] 2.6 Leave all comment padding and margins in `rem` — do not convert
      `.comment-thread`'s `0.75rem` inset or `.comment`'s padding to `em`
      (Decision 3, preserving hunk/comment/reply edge alignment).

## 3. Scale the comment-entry controls

- [x] 3.1 Change `.top-level-add-comment` (938) `font-size` to
      `calc(0.85rem * var(--zoom))`.
- [x] 3.2 Change `.overview-add-comment` (619) `font-size` to
      `calc(0.85rem * var(--zoom))`.
- [x] 3.3 Change `.modal-content textarea` (1148) `font-size` to
      `calc(1.05rem * var(--zoom))`.
- [x] 3.4 Change `.modal-content` (1127) `max-width` to
      `calc(600px * var(--zoom))`, leaving `width: 90%` in place as the on-screen
      clamp, and note in a comment why the pair works (Decision 4).
- [x] 3.5 Change `.modal-actions button` `font-size` to
      `calc(0.9rem * var(--zoom))`. Added at verify: all three modals in
      `index.html` are comment modals (add / edit / reply), so these are comment
      controls, and the spec's "no control sits at a fixed size beside text that
      has grown" covers them.
- [x] 3.6 Change `.modal-content h3` `font-size` to `calc(1.2rem * var(--zoom))`
      so the modal scales as a whole. Requested by the user at the second review
      gate, reversing the earlier call to leave the title fixed.

## 4. Leave alone, deliberately

- [x] 4.1 Make no change to `.outdated-line` / `.outdated-hunk` — the renderer
      emits `.outdated-line` with `.diff-line`, so it already scales at the code
      scale; adding a rule here would override and break it (Decision 5).
- [x] 4.2 Make no change to the dead `.comment-btn` (858) and `.line-actions`
      (847) rules (Decision 6).
- [x] 4.3 Make no change to chrome: header, sidebar, `.file-item`,
      `.comment-count` (518), toolbar buttons (Decision 7). This includes
      `#current-file-name`, the file-content header — confirmed fine as-is by the
      user at the second review gate.
- [x] 4.4 Make no change under `ts/` — the zoom handlers, clamp, and Ctrl+0 reset
      are untouched by this change.

## 5. Verify

5.1–5.6 are left unchecked deliberately: they are on-screen checks behind a
rebuild (`./manage.sh release.install`), which the user runs, not the agent. The
code work is complete and the checks that can be run headlessly (5.7) have been.

- [ ] 5.1 Rebuild and restart the app (the assets are `//go:embed`-compiled, so a
      refresh proves nothing), then open a file that has line-anchored comments,
      top-level comments, and outdated comments.
- [ ] 5.2 At zoom 1.0, confirm the rendering is unchanged from before the change —
      comment text, badge, author, and buttons at their existing sizes.
- [ ] 5.3 Ctrl+wheel in and out: confirm comment body text tracks the diff code in
      both directions, and that author, status badge, and action buttons stay
      proportionally smaller than the body text throughout.
- [ ] 5.4 Confirm all three thread contexts scale, and that the outdated block's
      captured hunk still scales as code.
- [ ] 5.5 At maximum zoom (3.0), open the comment modal: confirm the title, the
      typed text and the Save/Cancel buttons are all at the scaled size, the modal
      is wider than at 1.0, and it stays within the window. Check comment text
      wraps rather than overflowing its card.
- [ ] 5.6 Confirm the header, file list, and per-file comment-count badge have not
      moved at any zoom factor, then press Ctrl+0 and confirm everything returns to
      the base size.
- [x] 5.7 Run `deno task test` (or the project's equivalent) to confirm the
      frontend tests are unaffected, and `openspec validate
      scale-comment-text-with-zoom`.
