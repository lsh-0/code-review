## Why

A unified `git diff` only carries a few lines of context around each change, so a reviewer often cannot tell what surrounds a hunk — the enclosing function, an earlier guard clause, the rest of a struct. Today the tool discards everything outside the diff and offers no way to recover it, forcing the reviewer back to their editor to understand a change in context. Two related gaps compound this: a binary file renders as a confusing empty diff pane, and there is no way to jump from a changed file to the file itself in a real editor.

## What Changes

- Add a way to read the full content of a changed file at the review's source (head) and target (base) branches, on demand, rather than only the `git diff` output.
- Render an expansion affordance at each gap where the file has hidden lines: before the first hunk, between adjacent hunks, and after the last hunk.
- Clicking an affordance reveals more of the surrounding unchanged file in fixed-size steps (default 20 lines), growing the visible window without re-fetching the diff. Repeated clicks keep growing the window until the gap is exhausted.
- When the remaining gap between two hunks is smaller than one step, collapse the affordance into a single "expand all" that reveals the whole gap and visually joins the two hunks.
- Refuse to render a body for binary files: detect them from the diff's `Binary files ... differ` marker, still list them in the file navigation, but show a plain "binary file" placeholder when selected — no hunks, no expansion affordances, no blob fetching.
- Add a "browse" link in the filename line that opens the changed file in the OS-preferred application via `xdg-open` against the working-tree path.
- Preserve existing behaviour: line numbers, comment anchoring by new-file line number, and the clickable new-line gutter all continue to work on revealed context lines.

## Capabilities

### New Capabilities
- `diff-context-expansion`: revealing unchanged file lines around a diff hunk on demand (reading full file content from git at the base and head revisions and serving incremental ranges of context lines), refusing to render binary file bodies, and opening a changed file in the OS-preferred application.

### Modified Capabilities
<!-- None — no existing specs to modify. -->

## Impact

- **Backend (`backend/`)**: new git access to read a file's full content at a revision (`git show <rev>:<path>`); diff parsing gains a `Binary` flag set from the `Binary files ... differ` marker; new `App` methods, bound through Wails, that return a range of context lines for a file and open a working-tree file with `xdg-open`; the revisions (source/target branch) and repo path are already held on the review model/app.
- **Frontend (`frontend/web.go`)**: `renderDiff` and the hunk-rendering path gain gap affordances and the logic to request, splice, and re-render revealed lines; a binary file short-circuits to a placeholder; the filename line gains a "browse" link; selection/zoom/comment paths are unaffected.
- **Styling (`assets/style.css`)**: new classes for the expansion control rows, the binary placeholder, and the browse link.
- **No breaking changes**: the existing bound methods are unchanged; the new `Binary` field is additive to the diff JSON and the rest of the feature is additive.
