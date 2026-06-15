# TODO

A structured capture of deferred work, ideas, and nice-to-haves.
Each item is separated by `---` and has key-value metadata followed
by optional free-text context.

---
title: Ctrl-click multi-select files into a combined diff pane
added: 2026-06-12
effort: high
tags: frontend, ui
summary: Ctrl-clicking file items selects multiple files and renders them as one stacked pane on the right, each section keeping its filename header

Today selection is single-file: `currentFile` is a string,
`selectFile` (`frontend/web.go:275`) clears every `.active` item and
marks one, then `renderDiff(filePath)` (`frontend/web.go:335`) renders
that one file into `#diff-content`. The filename shows once in
`#current-file-name`.

Multi-select changes the shape: replace the single `currentFile` string
with an ordered selection set; a plain click selects just one (current
behaviour), ctrl/cmd-click toggles a file in/out of the set. Render the
combined view by iterating the selected files and, for each, emitting a
filename header followed by that file's hunks into `#diff-content` —
the per-file header moves from the single `#current-file-name` element
into the stacked content so every section is labelled.

Knock-on points: `selectFile`'s single-`.active` logic becomes
multi-`.active`; comment loading (`loadComments`, `frontend/web.go:304`)
and the `commentsCache` keyed by path must cover all selected files, not
just one; and the diff click/scroll handlers that assume a single
current file need auditing. Decide whether the selection set persists in
review state or is view-only.
---
title: Import review feedback from Bitbucket and GitHub
added: 2026-06-12
effort: high
tags: backend, integration, model
summary: Pull existing PR review feedback from Bitbucket and GitHub into the tool so it can be worked through locally

Work uses AI code review; the feedback is good but the platforms make
it awkward to address. Importing it here allows a better conversation
and thorough resolution using local tools.
---
title: Investigate syntax highlighting for diff code
added: 2026-06-12
updated: 2026-06-13
effort: medium
tags: frontend, ui
summary: Break up blocks of code with syntax highlighting for readability

Recommended options: Chroma (Go library) or Highlight.js (JavaScript).
A naive regex highlighter was tried but raised escaping and correctness
concerns, so a real library is likely the better path. Chroma already
appears in the module graph (go.work.sum), which de-risks choosing it.
---
title: Publish an AppImage and investigate ARM builds
added: 2026-06-13
effort: medium
tags: build, distribution
summary: Ship an AppImage for non-ARM users and explore producing an ARM binary for ARM-architecture users

Distribution today assumes a local build. An AppImage covers x86_64
users without a toolchain; ARM support needs investigation because of
the GopherJS/webkit constraints.
---
title: Give child comments parity with parent comments
added: 2026-06-13
effort: medium
tags: frontend, ui, comments
summary: Render replies more like top-level comments, including edit and delete buttons

Replies currently render as a lighter-weight thread; they should gain
the same affordances as parent comments, notably edit and delete.
---
title: Preserve page state when adding a comment
added: 2026-06-13
effort: medium
tags: frontend, ui, comments
summary: Submitting a comment discards expanded context lines and scrolls the window to an unfamiliar position

Adding a comment re-renders the diff, dropping expanded lines and other
page state, so the viewport jumps. The expand-context feature made this
more noticeable.
---
title: Disable the bottom "expand 20 lines" when a file ends at the last hunk
added: 2026-06-13
effort: low
tags: frontend, ui
summary: The end-of-file expand control renders enabled even with nothing below, only disabling itself after a click

The top affordance disables correctly, but the bottom one starts enabled
because end-of-file is only learned after the first fetch. It should be
disabled up front when there is nothing to reveal.
---
title: Add a right-click context menu
added: 2026-06-13
effort: medium
tags: frontend, ui
summary: No context menu on right-click; selecting lines and right-clicking offers no "copy" action

Right-clicking a selection in the diff offers nothing. A context menu
with at least a copy action is the motivating case.
---
title: Enlarge the add-comment box text
added: 2026-06-13
effort: low
tags: frontend, ui, comments
summary: Comment input text looks small and squished; a slightly larger size (~1.1–1.2x) may read better
---
title: Overview of all files and hunks with review feedback
added: 2026-06-13
effort: high
tags: frontend, ui, comments
summary: A view at the end of the files list that gathers every file and hunk carrying feedback into one stacked pane

Akin to the Ctrl-click compound-diff idea, but scoped to commented
content. For long, involved reviews an at-a-glance summary of everything
with feedback would help.
---
title: Top-level comments not attached to code
added: 2026-06-14
effort: high
tags: frontend, backend, model, comments
summary: Review comments unattached to any line, for overall feedback, with the same status and reply behaviour as code comments

Surfaced on the overview page alongside code comments. Same
resolve/ignore status and replies, just no line anchor. Needs state
schema changes and an update to the `_readme` review instructions.
---
title: Move review instructions into a markdown file
added: 2026-06-14
effort: low
tags: backend, model, docs
summary: Keep the review instructions as a markdown file, read in and stamped into the state file's `_readme` field

The instructions currently live as a hard-coded string constant. A
markdown file would be easier to edit and read; the build/runtime would
load it into the `_readme` field rather than embedding the prose inline.
---
title: Tighten the automated-review protocol, possibly via a CLI
added: 2026-06-14
effort: high
tags: backend, model, automation
summary: Sharpen how an agent uses the state to leave feedback (when to comment, when to unmark); consider a CLI instead of hand-editing JSON

Not about how to review code, but the mechanics of the agent's
interaction with the state. A `code-review` CLI offering direct actions
might be cleaner and less error-prone than editing the JSON by hand.
---
title: Instruct agents on feedback scope beyond the changes under review
added: 2026-06-14
effort: medium
tags: backend, model, automation
summary: Feedback often intends a project-wide change, but agents scope edits to just the changes under review

A comment like "use a convenience function for this emerging pattern"
usually means apply it everywhere, not only to recent changes. The
instructions should also favour showing the fix in a narrow scope first,
then confirming before applying project-wide.
---
title: Review the refresh implementation
added: 2026-06-14
updated: 2026-06-15
effort: medium
tags: frontend, backend
summary: Refresh has accreted fixes and feels hacky; review how it reloads state and rework it holistically

Refresh started as a partial reload and was patched repeatedly (marked
files, then the diff/file list). Worth a deliberate pass over the whole
refresh path rather than more piecemeal fixes.

Concrete defect found: `selectFile` calls `RefreshState` on every file
click, and commit `9e5311c` made `RefreshState` shell out to `git diff`
and re-parse, so each click now pays a synchronous subprocess — the
likely cause of the pause on first body display. State-reread-on-select
should reload only review state (comments/marks); the diff recompute
belongs on the explicit refresh button.
---
title: Don't reset scroll when clicking the already-selected file
added: 2026-06-14
effort: low
tags: frontend, ui
summary: Re-clicking the file you're already viewing re-renders the diff and resets scroll, mimicking a fresh page load when you're on the same page

Clicking a file's name should be a no-op when it's already the current
file, leaving scroll position untouched.
---
title: Move file selection with up/down arrow keys
added: 2026-06-14
effort: medium
tags: frontend, ui
summary: Up/down currently just scroll the file list; they should move the selection between files and keep the selected file in view

When navigating by keyboard, the file list should scroll to keep the
selected file visible where possible.
---
