# diff-context-expansion Specification

## Purpose
TBD - created by archiving change expand-diff-context. Update Purpose after archive.
## Requirements
### Requirement: Expansion affordances at hidden-line gaps

The diff viewer SHALL render an expansion affordance at the top of the first hunk, between adjacent hunks, and below the last hunk. The affordance is active when the file omits unchanged lines at that position (above the first hunk, in a non-contiguous gap between two hunks, or below the last hunk) and disabled when there are no hidden lines to reveal there, so the control stays present and predictable rather than appearing and disappearing.

#### Scenario: Hidden lines exist above the first hunk
- **WHEN** a file's first hunk begins below line 1 of the new file
- **THEN** an expansion affordance is shown above that hunk offering to reveal earlier lines

#### Scenario: Hidden lines exist between two hunks
- **WHEN** two adjacent hunks are separated by one or more unchanged lines not present in the diff
- **THEN** an expansion affordance is shown in the gap between them

#### Scenario: Hidden lines exist below the last hunk
- **WHEN** the file continues beyond the last line of the last hunk
- **THEN** an expansion affordance is shown below that hunk offering to reveal later lines

#### Scenario: No hidden lines at a file boundary
- **WHEN** the first hunk starts at line 1, or the last hunk ends at the final line of the file
- **THEN** the affordance for that boundary is still shown but rendered in a disabled state, so the control stays anchored at the top or bottom of the hunk even though it cannot reveal further lines

### Requirement: Incremental context expansion

Activating an expansion affordance SHALL reveal a fixed number of previously hidden unchanged lines (the expansion step, default 20) adjacent to the gap, and SHALL insert them in correct file order without re-fetching or altering the rest of the diff. Repeated activation SHALL continue revealing further lines until the gap is exhausted.

When a gap is fully revealed, the affordance SHALL settle into one of two terminal states: a **file-boundary** affordance (top of the first hunk, bottom of the last hunk) SHALL remain visible but disabled so navigation stays anchored and predictable, while a **between-hunks** affordance SHALL be removed because the two hunks have merged into one continuous block. A disabled affordance SHALL ignore activation.

#### Scenario: Reveal a step of lines below a hunk
- **WHEN** the reviewer activates a downward expansion affordance and the gap holds more lines than one step
- **THEN** the next step of unchanged lines is inserted immediately after the hunk, each carrying its correct old and new line numbers, and the affordance remains for further expansion

#### Scenario: Reveal a step of lines above a hunk
- **WHEN** the reviewer activates an upward expansion affordance and the gap holds more lines than one step
- **THEN** the previous step of unchanged lines is inserted immediately before the hunk, each carrying its correct old and new line numbers, and the affordance remains for further expansion

#### Scenario: Gap smaller than a full step
- **WHEN** the reviewer activates an expansion affordance and the remaining gap holds fewer lines than one step
- **THEN** all remaining lines in the gap are revealed, and the affordance then settles into its terminal state for that boundary: disabled-but-visible at a file boundary, or removed between two hunks

#### Scenario: File-boundary gap fully revealed
- **WHEN** expansion reveals the last hidden line above the first hunk or below the last hunk
- **THEN** the affordance remains in place but disabled, anchored at the top or bottom of the hunk, and ignores further activation

#### Scenario: Gap fully revealed between two hunks
- **WHEN** expansion reveals the last hidden line between two hunks
- **THEN** the two hunks become visually continuous, the lower hunk's `@@` header is hidden, and no expansion affordance remains between them

### Requirement: Full file content available at base and head revisions

The system SHALL provide the full content of a changed file at both the review's target (base) and source (head) revisions, so that unchanged lines outside the diff can be retrieved. Requesting content for a path or revision that does not exist SHALL fail without affecting the rest of the review.

#### Scenario: Retrieve context lines for an existing file
- **WHEN** the viewer requests a range of lines for a changed file that exists at the head revision
- **THEN** the system returns those lines from the head revision's full file content

#### Scenario: Requested file does not exist at the revision
- **WHEN** the viewer requests lines for a path that does not exist at the requested revision
- **THEN** the request reports an error and no expansion is rendered, while the rest of the diff remains usable

### Requirement: Revealed context preserves diff behaviour

Lines revealed by expansion SHALL behave as ordinary unchanged context lines: they display old and new line numbers, the new-line gutter remains clickable to add a comment, and any existing comment anchored to a revealed line's new-file line number SHALL appear once that line is shown.

#### Scenario: Comment on a revealed line
- **WHEN** a comment is anchored to a new-file line number that was hidden and is then revealed by expansion
- **THEN** the comment thread is displayed against that line after it is revealed

#### Scenario: Add a comment on a revealed line
- **WHEN** the reviewer clicks the new-line gutter of a revealed context line
- **THEN** the add-comment interaction opens for that line number exactly as it does for lines originally in the diff

### Requirement: Binary files are listed but not rendered

A file whose diff carries the `Binary files ... differ` marker SHALL be flagged as binary during diff parsing. A binary file SHALL still appear in the file navigation, but selecting it SHALL show an explanatory placeholder that states the file cannot be diffed — the text "binary file, cannot diff" — instead of a diff body, and SHALL NOT render hunks, expansion affordances, or fetch the file's blob content. The placeholder SHALL explain why there is no diff body rather than merely labelling the file as binary, so the empty body reads as expected rather than broken.

#### Scenario: Diff marks a file as binary
- **WHEN** a changed file's diff section contains `Binary files ... differ` and no hunks
- **THEN** that file is flagged as binary in the parsed diff

#### Scenario: Binary file appears in navigation
- **WHEN** the file list is rendered and one changed file is binary
- **THEN** the binary file is listed alongside text files

#### Scenario: Selecting a binary file
- **WHEN** the reviewer selects a binary file
- **THEN** an explanatory placeholder reading "binary file, cannot diff" is shown in place of a diff body, with no hunks and no expansion affordances, and no blob content is fetched for it

### Requirement: Open a changed file in the preferred application

The filename line for a changed file SHALL offer a "browse" action that opens the file at its working-tree path in the operating system's preferred application via `xdg-open`. Activating it SHALL NOT alter the review state, and a failure to open SHALL NOT disrupt the review.

#### Scenario: Browse a changed file
- **WHEN** the reviewer activates the browse action for a changed file
- **THEN** the file at its working-tree path is opened with `xdg-open` in the OS-preferred application

#### Scenario: Browse target cannot be opened
- **WHEN** activating the browse action fails (for example the path is absent or no opener is available)
- **THEN** the failure is reported without changing review state and the rest of the review remains usable

