# TODO

A structured capture of deferred work, ideas, and nice-to-haves.
Each item is separated by `---` and has key-value metadata followed
by optional free-text context.

---
title: Ctrl-click multi-select files into a combined diff pane
added: 2026-06-12
updated: 2026-06-18
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

Now somewhat easier: per-file hunks already load lazily (`ensureFileDiff`)
and `renderDiff` renders one file at a time, so the data side composes for
several files. The bulk of the work is the selection-model refactor
(`currentFile` to a set, multi-`.active`, append-not-replace rendering)
and auditing the single-file click/scroll handlers.
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
to the frontend. The external-change banner added since (show/hide plus
CSS) is now a reusable banner pattern to copy. Remaining: render the two
banners and wire the difftool link.
---
title: Build macOS and Windows binaries in CI and attach them to releases
added: 2026-06-18
effort: high
tags: build, distribution, ci
summary: Use GitHub Actions to cross-build the Wails app for macOS and Windows on tag, turn each tag into a GitHub release, and attach the artefacts

Distribution is local-build only today. The Wails cross-platform guide
(wails.io/docs/guides/crossplatform-build/) covers building per-OS in CI;
turning a pushed tag into a release with the binaries attached is the
companion piece. Relates to the AppImage item, which covers Linux.
---
