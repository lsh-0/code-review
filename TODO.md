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
title: Detect new commits on the source branch and offer a refresh banner
added: 2026-06-18
effort: medium
tags: backend, frontend, ui
summary: Watch the source-branch HEAD for new commits and show the refresh banner, as the state-file watcher already does for external review edits

The existing watcher polls only the review-state JSON, so an agent's
comment edits raise the banner but a new commit does not — the reviewer
sees no prompt that the diff is now stale. A separate poll of the
source-branch HEAD would close that gap and reuse the same banner.
---
title: Re-anchor comments to their lines across commits
added: 2026-06-18
effort: high
tags: backend, model, comments
summary: A comment's line number is fixed at the commit it was made against; when a later commit shifts that line the comment should follow it rather than point at the wrong line

A comment is correct for the commit it was left on, but a new commit that
inserts or removes lines above it pushes the referenced line out of place.
Comments already store surrounding-line context, which is the groundwork
for relocating the anchor when the diff changes.
---
title: Scale comment text with Ctrl+zoom like the diff text
added: 2026-06-20
effort: low
tags: frontend, ui
summary: Ctrl+zoom scales the diff code but leaves comment text at a fixed size; comments should scale with the same --zoom property

Pre-existing behaviour, unchanged by the Deno frontend rewrite; noticed
during that rewrite's webview check. The diff lines scale via the --zoom
custom property but the comment font sizes are fixed.
---
title: Sort and group the file list, surfacing files needing attention at the top
added: 2026-06-21
effort: medium
tags: frontend, ui
summary: Bring standard sorting/filtering/grouping to the file list; first goal is to lift files unmarked by a state-file change or new commit to the top so they aren't hunted for by scrolling

Over repeated review rounds the reviewer mainly wants the files that got
unmarked (by agent edits or new commits) without scrolling to find them.
Grouping by extension (review all SQL, or all shell, at once) is a likely
later refinement.
---
title: Spurious downward "expand lines" affordance below a fully-deleted file
added: 2026-06-20
effort: medium
tags: frontend, ui, diff
summary: A wholly-removed file (hunk `@@ -1,3 +0,0 @@`) shows a "↓ expand 20 lines" control below its last line, but there is nothing below to expand into

Seen on `assets/go.mod` shown as deleted. The downward expand control
should be suppressed when the hunk already reaches the end of the file,
or when the file no longer exists on the new side.
---
title: Empty diff pane gives no reason for renamed or binary files
added: 2026-06-20
effort: medium
tags: frontend, ui, diff
summary: Pure-rename (no content change) and binary files render a blank diff pane with no indication of why — should show "renamed, no changes" or "binary file" instead

A run of files rendered empty: `backend/assets/assets.go` (a rename with
no content change) and the `.otf` fonts (binary) show nothing in the diff
pane, while `style.css` (rename plus real edits) renders. The reviewer
can't tell whether the file genuinely has no changes, is binary, or the
tool failed. Each case wants its own placeholder message.
---
title: Overview keeps an empty file section after its last comment is deleted
added: 2026-06-23
effort: medium
tags: frontend, ui, overview, comments
summary: Deleting a comment in the review overview can leave the file's header showing with no hunk beneath it; a file with no remaining feedback should drop out of the overview entirely

The overview gathers files by their feedback, so once a file's last comment
is removed it has nothing to show and should disappear rather than leave a
stranded header.
---
title: Ctrl-scrolling in the file list resizes the wrong pane via zoom
added: 2026-06-23
effort: low
tags: frontend, ui
summary: Ctrl-scroll inside the file list pane changes the zoom/size of the diff pane; it should be contained or do nothing there
---
title: Run the controls horizontally across the top instead of stacked above the file list
added: 2026-06-24
effort: medium
tags: frontend, ui
summary: Redesign the controls to sit in a horizontal bar across the top of the window rather than stacked vertically on top of the file list column
---
