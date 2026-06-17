# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- A banner across the top of the window when another program (such as an AI agent or a command-line tool) changes the
  review while it is open, offering a refresh. The view never changes on its own; the reviewer chooses when to reload.

### Changed

- Selecting a file no longer recomputes the diff, so switching between files is immediate. The diff is now recomputed
  only when Refresh is pressed, where newly committed files are picked up.
- Adding, resolving or deleting a comment updates just that comment's thread in place, keeping expanded context lines and
  scroll position rather than re-rendering the whole file.
- A file's "done" mark now records the version it was reviewed against, so the mark is dropped whenever a commit changes
  that file — including changes made while the app was closed — returning it to the set of files still needing review.
  - The `marked_files` field in the state file changes from a list of paths to a list of `{path, blob}` records; existing
    state files are upgraded automatically on first open.

## [0.6.0] - 2026-06-17

### Added

- A "Review overview" pane, reached from a button pinned to the bottom of the file list, that stacks every commented file
  and its feedback into one view so a long review can be taken in at a glance; clicking a file's heading opens it.
- Review-level comments for overall feedback, created from the overview and not anchored to any file or line. They behave
  like code comments — status, replies, edit and delete — and are stored in a top-level `comments` array in the state file.

### Changed

- Replies are now first-class comments: each carries a parent reference and gains the same edit and delete actions as
  top-level comments.
  - Replies are stored flat in the state file with a `parent_id`; existing state files lose their old nested replies.
- The state file's `_readme` instructions now describe the full review-handling policy, including telling an AI agent to
  unmark any file it changes so the reviewer can see what needs revisiting.
- Refreshing a review reloads marked files and recomputes the diff, so newly committed files appear without restarting.
- Comment input text enlarged for readability.
- Release builds default to an "unreleased" version when none is supplied.

### Fixed

- The bottom "expand 20 lines" control is disabled up front when a hunk already reaches the end of the file, instead of
  only after a click reveals there is nothing below.
- Re-clicking the file already being viewed no longer re-renders the diff and resets the scroll position.
- Unchanged context between two hunks can now be revealed from either side: each gap has both an upward and a downward
  "expand" control that converge and merge the hunks when they meet. Previously only the file's last hunk had a downward
  control, and a hunk ending in deletions could wrongly disable it.

## [0.5.0] - 2026-06-13

### Added

- Expand the diff to show more of the surrounding file: an "expand 20 lines" control sits above, between and below hunks
  and reveals hidden unchanged context on demand.
  - Fully revealing the gap between two hunks merges them into one continuous block.
  - At the top or bottom of a file the control stays visible but disabled, so the navigation never shifts.
- Threaded replies on review comments, persisted with each comment in the state file.
- Per-file "done" checkbox in the file list, also toggled by double-clicking a file; the marked state is remembered.
- "Browse" link on the filename that opens the file in the operating system's preferred application.

### Changed

- Diff lines now wrap on word boundaries rather than mid-word, with long unbreakable tokens still wrapping to avoid
  horizontal overflow.

### Fixed

- Binary files are detected and shown as a plain "binary file" placeholder instead of a confusing empty diff.

## [0.4.0] - 2026-06-11

### Added

- Zoom the diff text with Ctrl+scroll or Ctrl with the plus, minus and zero keys.
- "Copy AI prompt" button that copies a ready-made prompt pointing a tool at the review's state file; the state file now
  carries a self-describing `_readme` field explaining its schema and how to act on comments.

### Changed

- New typography and palette: Atkinson Hyperlegible Next for the interface and Source Code Pro for code, on a warmer,
  lower-glare background, with a larger default text size for legibility.
- Side navigation kept cooler than the body to separate the file list from the diff.
- Selecting and copying across diff rows no longer includes the line numbers.
- Hovering a multi-line diff selection shows a left-edge row indicator rather than recolouring the line.

## [0.3.0] - 2026-06-11

### Changed

- Backend and supporting modules now build on Go 1.26; the GopherJS frontend and shared model remain on Go 1.19, the
  highest version GopherJS supports.
- Desktop application now builds against webkit2gtk-4.1 (libsoup3), since the older webkit2gtk-4.0 packages are no longer
  maintained against current system libraries.
- `manage.sh lint` now tidies, formats, vets and fixes each module rather than only formatting.
- README documents installing and running the tool via `manage.sh release.install`.

## [0.2.0] - 2026-04-09

### Added

- Refresh button to reload review state on demand; state is also re-read whenever a file changes.

### Changed

- Colour values moved into CSS variables for consistent theming.

## [0.1.0] - 2026-02-10

### Added

- Initial desktop code-review tool: a two-pane diff viewer for reviewing changes between branches in a local repository.
- Inline commenting by clicking a line number, with Esc to dismiss the dialog; surrounding line context is recorded so
  comments can survive later changes to the file.
- Comments now carry an author, shown alongside the comment when it differs from the configured user.

### Changed

- Default branch is detected by looking for `main` or `master`.
- Smooth scrolling disabled and sluggish CSS transitions removed for a more responsive feel.

### Fixed

- `manage.sh` skips the WebKitGTK dependency check on macOS, which uses native WebKit.
