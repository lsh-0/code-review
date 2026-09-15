## ADDED Requirements

### Requirement: A top-level file comment is an anchorless comment on a file

The system SHALL support a **top-level file comment**: review feedback attached to a
file but to no line. A top-level file comment SHALL be a root comment stored in that
file's comment list carrying no anchor — the same anchorless shape used by review-level
overall comments. Because it has no anchor, its current line number SHALL be `0` and it
SHALL NOT be outdated. A top-level file comment SHALL support the same status, reply,
edit, and delete actions as any other file comment, and SHALL persist to the review
state file like any other comment.

#### Scenario: A top-level file comment carries no anchor

- **WHEN** a reviewer adds a top-level comment to a file
- **THEN** the comment is stored as a root comment in that file with an empty anchor history
- **AND** its current line number is `0`
- **AND** it is not outdated

#### Scenario: A top-level file comment is distinct from a review-level comment

- **WHEN** a top-level file comment and a review-level overall comment both exist
- **THEN** the file comment belongs to its file's comment list (carrying that file path)
- **AND** the review-level comment carries no file path

### Requirement: Adding a top-level file comment

The system SHALL let a reviewer add a top-level comment to any changed file through the
familiar add-comment modal, reached from an "Add comment" control at the top of that
file's view. Saving SHALL route to a backend mutation that appends an anchorless comment
to the file and returns the review's updated comments for that file, so the new comment
appears without recomputing the diff or reloading the whole view.

#### Scenario: Add a top-level comment to a file

- **WHEN** the reviewer activates the file's top-of-file "Add comment" control and saves non-empty text
- **THEN** an anchorless comment with that text is appended to the file's comments
- **AND** the file view updates incrementally to show the new comment at the top of the file, without recomputing the diff

#### Scenario: Empty comment text adds nothing

- **WHEN** the reviewer opens the add-comment modal for a file and saves empty text
- **THEN** no comment is added

### Requirement: Top-level comments render at the top of the single-file view

The system SHALL render a file's top-level comments at the top of its single-file view,
after the file header and before any code, together with the "Add comment" control.
Top-level comments SHALL render as ordinary comment threads (status, replies, edit,
delete). When a file has both top-level comments and untethered (outdated) comments, the
order at the top of the file SHALL be top-level comments first, then the untethered
comments block, then the code.

#### Scenario: Top-level comments appear above the code

- **WHEN** a file with a top-level comment is rendered in the single-file view
- **THEN** the top-level comment thread appears at the top of the file, above the first hunk
- **AND** an "Add comment" control is present at the top of the file

#### Scenario: Ordering with untethered comments

- **WHEN** a file has both a top-level comment and an outdated (untethered) comment
- **THEN** the top-level comment renders above the untethered comments block
- **AND** the untethered comments block renders above the code

### Requirement: Binary files support top-level comments

The system SHALL offer the top-of-file "Add comment" control and render existing
top-level comment threads for a binary file, whose explanatory "binary file, cannot
diff" placeholder has no lines to comment on. Adding or displaying a top-level comment on
a binary file SHALL NOT fetch or render a diff body for it.

#### Scenario: Comment on a binary file

- **WHEN** the reviewer selects a binary file
- **THEN** the "binary file, cannot diff" placeholder is shown together with an "Add comment" control and any existing top-level comment threads
- **AND** no blob content or diff body is fetched for the binary file

#### Scenario: A binary file's top-level comment persists and re-displays

- **WHEN** a binary file has a saved top-level comment and is reopened
- **THEN** the comment renders at the top of the binary file's view, above the placeholder body, with no diff body fetched

### Requirement: The overview shows a file's top-level comments

The review overview SHALL show a file's top-level comments within that file's overview
section, alongside the file's other feedback. A file whose only feedback is a top-level
comment SHALL still appear in the overview, including a binary file.

#### Scenario: Overview lists a file with only a top-level comment

- **WHEN** the overview is rendered and a file has only a top-level comment (and no line comments)
- **THEN** that file appears in the overview with its top-level comment thread shown

#### Scenario: Overview lists a binary file with a top-level comment

- **WHEN** the overview is rendered and a binary file has a top-level comment
- **THEN** the binary file appears in the overview with its top-level comment thread shown
