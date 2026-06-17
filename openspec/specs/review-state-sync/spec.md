# review-state-sync Specification

## Purpose
Keep the GUI's view of a review in step with its two sources of truth — the git diff and the review
state JSON — while paying each source's cost only when needed. The diff (expensive, a git subprocess)
is recomputed only on explicit refresh; review state (cheap, a JSON read) is reloaded when switching to
the overview; file selection reads already-loaded state and does neither. A reviewer's own comment
actions update only the affected thread, preserving expanded context and scroll.

## Requirements
### Requirement: Separate review-state reload from diff recompute

The system SHALL expose two distinct backend operations: a review-state reload that re-reads the
state JSON without invoking git, and a diff recompute that runs `git diff` and re-parses the result.
Neither operation SHALL do the other's work.

#### Scenario: Reloading review state does not invoke git

- **WHEN** the review-state reload operation runs
- **THEN** the state JSON is re-read into memory
- **AND** no git subprocess is invoked

#### Scenario: Recomputing the diff runs git

- **WHEN** the diff recompute operation runs
- **THEN** `git diff` between the review's branches is executed and parsed
- **AND** the file-content cache is reset

### Requirement: File selection does not recompute the diff

The system SHALL render a selected file from already-loaded state. Selecting a file SHALL NOT trigger
a review-state reload or a diff recompute.

#### Scenario: Selecting a file pays no git cost

- **WHEN** the reviewer clicks a file in the file list
- **THEN** the file's diff and comments render from in-memory state
- **AND** no git subprocess runs as a result of the selection

#### Scenario: Re-selecting the current file is a no-op

- **WHEN** the reviewer clicks the file that is already selected and the overview is not active
- **THEN** the view is left unchanged, preserving scroll position and expanded context

### Requirement: Explicit refresh recomputes the diff

The system SHALL, when the reviewer activates the explicit Refresh control, perform both the
review-state reload and the diff recompute, so that newly committed files appear and external state
edits are absorbed.

#### Scenario: Refresh absorbs new commits

- **WHEN** new commits have landed since the view was loaded and the reviewer activates Refresh
- **THEN** the diff is recomputed and the file list reflects the newly changed files

### Requirement: Opening the overview reloads review state only

The system SHALL reload review state, without recomputing the diff, when the reviewer opens the review
overview, so the overview reflects comment changes without paying the git cost.

#### Scenario: Overview reflects current comments

- **WHEN** the reviewer opens the review overview
- **THEN** review state is reloaded from the JSON
- **AND** the overview renders every commented file's current feedback
- **AND** no diff recompute occurs

### Requirement: Comment actions update the view incrementally

The system SHALL, when the reviewer adds, edits, resolves, ignores, reactivates, replies to, or
deletes a comment, return only the affected comment thread and patch that single DOM subtree, rather
than re-rendering the whole file or overview. Expanded context lines and scroll position SHALL be
preserved across the action.

#### Scenario: Adding a comment preserves expanded context and scroll

- **WHEN** the reviewer has expanded context lines and scrolled, then adds a comment
- **THEN** only the affected thread is rendered into place
- **AND** the previously expanded context lines remain visible
- **AND** the scroll position is unchanged

#### Scenario: Resolving a comment updates only its thread and the file status

- **WHEN** the reviewer resolves a comment
- **THEN** the comment's thread re-renders with the resolved status
- **AND** the file's comment-status indicator updates
- **AND** no other thread or file is re-rendered

#### Scenario: Deleting a comment removes only its thread

- **WHEN** the reviewer deletes a comment
- **THEN** that comment's thread is removed from the DOM
- **AND** the rest of the view is left intact
