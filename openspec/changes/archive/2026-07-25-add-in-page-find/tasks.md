## 1. Pure find core

- [x] 1.1 In `ts/core/find.ts`, implement `findMatches(haystack, needle)` returning the ordered, non-overlapping, case-insensitive match ranges (`{ start, length }`) over the original-case string; an empty or whitespace-only needle returns no matches
- [x] 1.2 Implement `nextMatchIndex(current, count)` and `prevMatchIndex(current, count)` stepping an index with wraparound, handling an empty list and an out-of-range `current`
- [x] 1.3 In `ts/core/find_test.ts`, unit-test (no DOM) case-insensitivity, ordering, original-case ranges, empty/whitespace term, non-overlap, and the wraparound / edge cases of the index steppers

## 2. Find bar render layer

- [x] 2.1 In `ts/render/find.ts`, build the find bar DOM (text input, "n of m" counter, previous / next / close controls) and mount it; expose `openFind`, `closeFind`, `findIsOpen`, `findNext`, `findPrev`
- [x] 2.2 `openFind` creates the bar (or re-focuses and selects the input if already open), rebuilding if the tracked bar is no longer in the live document, and focuses the input
- [x] 2.3 Highlight matches across the whole page: collect text nodes under `#app` (skip nodes already inside a find mark, inside a hidden subtree, and whitespace-only nodes), then wrap each match in a `<mark class="find-match">`, recording the marks in document order. The find bar and modals sit outside `#app`, so the bar's own controls are not searched
- [x] 2.4 Track the active match, applying the active-match class and `scrollIntoView`, and step it via the core index helpers on next / previous (`findNext` / `findPrev`); update the counter ("n of m", "No matches", or empty)
- [x] 2.5 `closeFind` clears highlights, restores the page DOM (replace marks with text and re-merge adjacent text nodes via an explicit `normalize()` stand-in), removes the bar, and returns focus to the diff
- [x] 2.6 Guard the methods the deno-dom shim lacks (`focus`, `select`, `scrollIntoView`) so the flow runs under both test and WebKitGTK
- [x] 2.7 In `ts/render/find_dom_test.ts`, DOM integration tests: marks inserted for matches, active-match selection and counter, stepping with wraparound, whole-page scope (the file list IS searched, hidden subtrees are NOT), `findNext` / `findPrev` stepping (including when closed), and that close restores the original text with no residual or nested markup across repeated searches

## 3. Keyboard wiring

- [x] 3.1 In `ts/main.ts`, bind Ctrl-f to `openFind` (preventing the default); F3 / Shift+F3 to `findNext` / `findPrev` when the bar is open and to `openFind` when it is closed; and Escape to `closeFind` when the find bar is open and focus has left it; wire Enter / Shift+Enter and Escape within the bar's own input

## 4. Styling and focus target

- [x] 4.1 In `backend/assets/style.css`, style the find bar (overlay pinned top-right of the window) and the match highlights, with the active match visually distinct
- [x] 4.2 In `backend/assets/index.html`, add `tabindex="-1"` to `#diff-view` so closing the bar can return focus to the diff

## 5. Verification

- [x] 5.1 Run `deno check`, `deno lint`, `deno fmt`, and the frontend tests; confirm all pass
- [x] 5.2 Regenerate the bundle (`deno task bundle`) and build the Go backend; confirm it builds and tests pass
- [x] 5.3 Manually verify in the WebKitGTK GUI: Ctrl-f opens the bar, matches highlight and scroll into view on step (Enter/F3), file-list matches highlight, and closing restores focus to the diff (confirmed by the user: "works a treat")
