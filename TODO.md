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
updated: 2026-06-29
effort: low
tags: frontend, ui
summary: The WebKit scrollbar is now always shown via `::-webkit-scrollbar` styling in `style.css`; remaining work is widening it

WebKitGTK honours `::-webkit-scrollbar`, so the styling control question is
answered and the always-visible half is done. The thumb still uses default
sizing — only an explicit width/height bump remains.
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
title: Hide files matching ignore patterns, with a show-hidden toggle
added: 2026-06-16
updated: 2026-06-29
effort: medium
tags: frontend, backend
summary: Keep a list of file patterns to always hide from the review (e.g. compiled .qtpl.go), with a way to reveal which files were hidden

Generated or compiled files (like .qtpl.go from .qtpl) are noise the
reviewer never cares about. A configurable ignore list would hide them;
a "show hidden" control would reveal which files were suppressed.

An ad-hoc substring filter box now exists (`fileMatchesFilter` in
`ts/core/filter.ts`), but that is a transient search, not a persistent
ignore-pattern list. The pattern list and show-hidden toggle remain.
---
title: Warn about uncommitted working-tree changes with banners
added: 2026-06-17
updated: 2026-06-29
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

Backend groundwork still in place and now wired through to the frontend:
`GetWorkingTreeStatus` (`backend/gitquery.go`) is bound and exposed as
`getWorkingTreeStatus()` in `ts/client.ts`. The reusable show/hide banner
pattern also exists (the external-change banner in `ts/main.ts`).
Remaining: render the two banners and wire the difftool link — none of
the working-tree status is consumed by the UI yet.
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
title: Scale comment text with Ctrl+zoom like the diff text
added: 2026-06-20
effort: low
tags: frontend, ui
summary: Ctrl+zoom scales the diff code but leaves comment text at a fixed size; comments should scale with the same --zoom property

Pre-existing behaviour, unchanged by the Deno frontend rewrite; noticed
during that rewrite's webview check. The diff lines scale via the --zoom
custom property but the comment font sizes are fixed.
---
title: Empty diff pane gives no reason for renamed or binary files
added: 2026-06-20
updated: 2026-06-29
effort: medium
tags: frontend, ui, diff
summary: Pure-rename (no content change) files render a blank diff pane with no indication of why — should show "renamed, no changes"

The binary case is now handled: `ts/render/hunks.ts` renders a "binary
file, cannot diff" placeholder (commit `ab7f9ca`). Pure-rename files
still render empty — the `DiffFile` type carries no `Renamed` field, so
rename detection has to be added before a "renamed, no changes"
placeholder can be shown.
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
