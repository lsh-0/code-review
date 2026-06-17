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

Today selection is single-file: `currentFile` is a string, `selectFile`
clears every `.active` item and marks one, then `renderDiff` renders that
one file into `#diff-content`. The filename shows once in
`#current-file-name`.

Multi-select changes the shape: replace the single `currentFile` string
with an ordered selection set; a plain click selects just one (current
behaviour), ctrl/cmd-click toggles a file in/out of the set. Render the
combined view by iterating the selected files and, for each, emitting a
filename header followed by that file's hunks into `#diff-content` —
the per-file header moves from the single `#current-file-name` element
into the stacked content so every section is labelled.

Knock-on points: `selectFile`'s single-`.active` logic becomes
multi-`.active`; comment loading (`loadComments`) and the `commentsCache`
keyed by path must cover all selected files, not just one; and the diff
click/scroll handlers that assume a single current file need auditing.
Decide whether the selection set persists in review state or is
view-only.
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
title: Add a right-click context menu
added: 2026-06-13
effort: medium
tags: frontend, ui
summary: No context menu on right-click; selecting lines and right-clicking offers no "copy" action

Right-clicking a selection in the diff offers nothing. A context menu
with at least a copy action is the motivating case.
---
title: Add a code-review CLI for agents instead of hand-editing JSON
added: 2026-06-14
updated: 2026-06-16
effort: high
tags: backend, model, automation
summary: A `code-review` CLI offering direct actions (resolve, reply, unmark) would be cleaner and less error-prone than an agent editing the state JSON by hand

The instruction prose on the agent's interaction with the state (when to
reply, when to unmark, working unsupervised) was sharpened in
`statefile-usage.md`. The remaining piece is the CLI: direct actions
would be less error-prone than hand-editing the JSON.
---
title: Move file selection with up/down arrow keys
added: 2026-06-14
effort: medium
tags: frontend, ui
summary: Up/down currently just scroll the file list; they should move the selection between files and keep the selected file in view

When navigating by keyboard, the file list should scroll to keep the
selected file visible where possible.
---
title: Make the scrollbar always-present and wider
added: 2026-06-16
effort: medium
tags: frontend, ui
summary: Investigate how much control we have over the scrollbar; want it always visible and wider, likely a WebKit styling concern

The scrollbar behaves differently to the GTK scrollbars, suggesting it
is WebKit-controlled rather than the system widget. Investigate how much
styling control WebKit exposes.
---
title: Ctrl-z undo doesn't work in the add-comment modal
added: 2026-06-16
effort: medium
tags: frontend, ui, comments
summary: Pressing Ctrl-z while typing in the add-comment box does not undo the edit
---
title: Ctrl-f should search the page contents
added: 2026-06-16
effort: medium
tags: frontend, ui
summary: Pressing Ctrl-f does nothing; it should search the page for matches and highlight them
---
title: Unmark files changed by new commits on refresh
added: 2026-06-16
updated: 2026-06-18
effort: medium
tags: backend, frontend
summary: On refresh, reconcile the marked-file list against new commits since the current view, dropping files that changed or were deleted

When new commits land, a file the reviewer marked may have changed and
needs revisiting, so it should drop off the marked list; deleted files
should drop too. Newly added files simply never appear there.

Partly done: `RecomputeDiff` now reconciles marks against the diff-delta
within a running session. The between-session case (file changed while
the app was closed) is the subject of the `durable-file-marks` OpenSpec
change, which stores a git blob SHA per mark; close this item once that
lands.
---
title: Make the file-list/diff divider draggable to resize the panes
added: 2026-06-16
effort: medium
tags: frontend, ui
summary: The divider between the file list and the diff cannot be dragged; clicking and dragging it should resize the two panes

The boundary between the file-list column and the diff pane is fixed.
Dragging it should adjust how the horizontal space is split between them.
---
title: Hide files matching ignore patterns, with a show-hidden toggle
added: 2026-06-16
effort: medium
tags: frontend, backend
summary: Keep a list of file patterns to always hide from the review (e.g. compiled .qtpl.go), with a way to reveal which files were hidden

Generated or compiled files (like .qtpl.go from .qtpl) are noise the
reviewer never cares about. A configurable ignore list would hide them;
a "show hidden" control would reveal which files were suppressed.
---
title: Warn about uncommitted working-tree changes with banners
added: 2026-06-17
updated: 2026-06-18
effort: medium
tags: frontend, backend, ui
summary: A full-width info banner counting modified/deleted tracked files, plus a per-file warning banner (with a link to open the file in the reviewer's diff tool) when the viewed file has local changes not reflected in the diff

Tracked files that changed or were deleted should trigger a friendly
green page-wide banner ("uncommitted changes detected: N modified, N
deleted"); new untracked files are ignored. Viewing a file with local
changes also shows an orange per-file banner ("file contains uncommitted
changes") with an inline link to open it in the reviewer's configured
diff tool — via `git difftool` (honours their `diff.tool`, e.g. meld),
since `xdg-open` opens a single file and cannot diff.

Backend groundwork done: `GetWorkingTreeStatus` (`backend/gitquery.go`)
returns tracked modified/deleted files (untracked excluded) and is bound
to the frontend. Remaining: render the two banners and wire the difftool
link.
    ---
    title: Profile slow startup and Refresh-button pauses
    added: 2026-06-18
    effort: medium
    tags: backend, performance 
    summary: The ~5s startup pause and ~2s Refresh pause both stem from the synchronous git diff; profile and consider moving it off the UI path

    Splitting RefreshState removed the per-file-click git cost (file
    selection is now instant), but two pauses remain: ~5s on app startup
    (RecomputeDiff runs a single git diff before the window paints, plus
    Wails/WebKit init) and ~2s on the Refresh button (the same git diff,
    now isolated to that explicit action). Profile to attribute the cost,
    then consider running the diff off the UI path or showing progress.
    ---

