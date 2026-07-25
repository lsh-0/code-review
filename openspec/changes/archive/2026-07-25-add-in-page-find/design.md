## Context

The GUI runs inside WebKitGTK, which does not expose the browser's native Ctrl-f
find. Because that native find is what users expect, the replacement stays as
close to it as possible: a browser searches the whole document, so this searches
the whole application page (`#app`) — diff, file list, and header — not just the
diff. The frontend already follows a three-layer split (pure core → typed RPC
client → render layer; see the `frontend-architecture` spec), and the page's
lines carry syntax-highlight spans. An in-page find must fit that split, search
the rendered page without disturbing it, and leave the DOM exactly as it found it
on close.

Note: an initial implementation existed from a prior lap; this design was written
to record it, then revised after the review gate to widen the search from the diff
to the whole page and add F3 / Shift+F3 stepping, per the reviewer's direction.

## Goals / Non-Goals

**Goals:**
- Replace the missing native Ctrl-f with an equivalent in-page find over the
  whole page (diff, embedded comments, file list, header), matching what native
  find does over the document.
- Support the common find idioms: Ctrl-f to open, type to search, F3 / Shift+F3
  (and Enter / Shift+Enter) to step through results.
- Keep the match-locating and selection-stepping logic pure and unit-testable
  without a DOM; keep all DOM manipulation in the render layer with integration
  tests.
- Highlight matches in place, step the active match with wraparound and scroll-
  into-view, and restore the DOM untouched on close.

**Non-Goals:**
- Regular-expression, whole-word, or persisted match-case options — find is a
  case-insensitive substring search.
- Searching hidden content (e.g. the collapsed change banner) — like native find,
  only shown text is matched.
- Joining a match that straddles two syntax-highlight tokens into one highlight.
- Replacing or replace-all functionality.

## Decisions

**Split pure logic from DOM work along the existing layers.**
`ts/core/find.ts` owns the decisions: `findMatches(haystack, needle)` returns the
ordered, non-overlapping, case-insensitive match ranges over the original string,
and `nextMatchIndex` / `prevMatchIndex` step an index with wraparound. These are
pure and unit-tested with no DOM. `ts/render/find.ts` owns the bar DOM, the
`<mark>` highlighting, the active-match tracking, and scroll-into-view. Rationale:
matches the `frontend-architecture` contract and lets the tricky range logic be
tested directly rather than through the DOM. Alternative considered: doing the
search inline in the render layer — rejected because it would make the core logic
untestable without a browser.

**Search the whole page (`#app`), not just the diff.**
The search root is `#app`, the application content wrapper. The find bar and the
comment modals are appended outside `#app` (siblings on `document.body`), so
scoping to `#app` naturally covers the visible page — diff, file list, header —
while excluding the bar's own controls (so its counter text can't self-match) and
the modals. Hidden subtrees (elements carrying the `hidden` class, e.g. the
collapsed change banner) are skipped so they contribute no phantom matches, the
way native find ignores non-displayed text. Rationale: this is what the user
expects from Ctrl-f — the file list already has a filter box, but a find that
silently ignored two-thirds of the page would not read as "find".

**Highlight per text node, wrapping matches in `<mark>`.**
The render layer collects the text nodes under the search root (skipping nodes
already inside a find mark, inside a hidden subtree, and whitespace-only nodes),
then replaces each node that contains a match with a fragment of plain text and
`<mark class="find-match">` spans. Collecting nodes before mutating avoids
re-walking inserted marks. Rationale: text-node granularity composes with the
existing syntax-highlight spans without re-parsing the page. Trade-off: a match
split across two adjacent highlight tokens lands in two text nodes and is not
joined into one highlight — rare per line, and documented in code.

**Restore the DOM explicitly rather than relying on `normalize()`.**
On close (or before each re-run) every mark is replaced by a text node of its
content and the touched parents have their adjacent text nodes re-merged by an
explicit `mergeAdjacentTextNodes`, standing in for the DOM `normalize()` that the
deno-dom test shim does not implement. Rationale: keeps repeated searches correct
regardless of host support, so a term straddling a former mark's seam is still
found. Alternative considered: calling `normalize()` — rejected because it breaks
under the test shim and silently depends on host behaviour.

**Keyboard wiring lives in `main.ts`; the bar owns its own input keys.**
`main.ts` binds the global keys — Ctrl-f (open), F3 / Shift+F3 (step next /
previous, or open the bar when it is closed), and Escape (close when focus has
left the bar) — driving the render layer through `openFind` / `findNext` /
`findPrev` / `closeFind`. The bar's input additionally handles Enter / Shift+Enter
(step) and Escape (close) while it has focus. F3 is global so it works as the
familiar find-again key regardless of where focus sits. Rationale: keeps the
global handler thin and the bar self-contained while covering the standard find
idioms.

**Module-private singleton state.**
The render layer holds the open bar's state (input, counter, marks, active index)
in a module-private variable exposed through `openFind` / `closeFind` /
`findIsOpen`, so `main.ts` drives it without threading a controller around. A
tracked bar no longer in the live document is treated as stale and rebuilt.

## Risks / Trade-offs

- Match straddling a syntax-highlight token boundary is not highlighted as one
  span → accepted; rare per line, called out in a code comment, and the match is
  still counted within each token it falls in.
- WebKitGTK-specific behaviour (actual scroll-to-match and focus return) cannot
  be exercised by the deno-dom tests → mitigated by guarding the shim-unsupported
  methods (`focus`, `select`, `scrollIntoView`) and flagged for a manual check at
  the GUI on review.
