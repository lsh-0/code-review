# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
