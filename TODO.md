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
title: Expand diff context to show more surrounding file
added: 2026-06-12
effort: medium
tags: frontend, ui
summary: Let the viewer grow a hunk's visible window to reveal more of the unchanged file around a snippet, for better context

Misses GitHub's expandable diff context — the control that grows the
window to expose hidden surrounding lines.
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
