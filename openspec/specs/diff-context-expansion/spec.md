# diff-context-expansion Specification

## Purpose
Present a changed file as windows onto its content: contiguous ranges that default to one per hunk,
grow on demand, merge when the hidden gap between them closes, and collapse back to their defaults on
refresh. A comment anchored outside every hunk gets a default window of its own, so it is visible
without manual expansion. Window state (growth, revealed lines) lives in the frontend's view state and
survives switching files within a session; rendering a file's region is always a function of that
state, never of DOM nodes accumulated by prior expansion.
## Requirements

### Requirement: A file's diff renders as windows onto the file

The viewer SHALL present a changed file as a set of **windows**: contiguous, non-overlapping ranges
of new-side line numbers, ordered by position, separated by hidden gaps. Within a window, lines that
belong to a hunk SHALL render as that hunk's diff lines — including removed (old-side) lines
interleaved at their positions — and lines outside every hunk SHALL render as ordinary unchanged
context lines drawn from the file's content at the head revision.

The default windows for a file SHALL be: one window per hunk, spanning the hunk's natural range; plus
one window per line-anchored comment whose line falls outside all hunk windows, spanning that line
and the ordinary diff context (3 lines) on each side, clamped to the file's bounds. Several comments
in one gap MAY yield a single merged window. An untethered comment (outdated, or unplaceable) SHALL
NOT produce a window.

#### Scenario: Hunks render as default windows

- **WHEN** a file with two hunks and no outside-hunk comments is opened
- **THEN** the file renders as two windows, one per hunk, with a hidden gap between them

#### Scenario: A comment outside every hunk gets its own window

- **WHEN** a file is opened and a comment is anchored to an unchanged line far from every hunk
- **THEN** a window spanning the comment's line and the ordinary diff context around it renders at
  its position in the file, with the comment thread against its line
- **AND** the lines between that window and the neighbouring hunks stay hidden

#### Scenario: Window content merges hunk lines and file lines

- **WHEN** a window covers both a hunk and lines beyond the hunk's range
- **THEN** the hunk's portion renders its diff lines, removed lines included, and the remainder
  renders unchanged context lines from the head revision, each with correct old and new line numbers

### Requirement: Windows merge when the gap between them closes

Two adjacent windows SHALL merge into one when the hidden gap between them falls below a small
threshold (3 lines): the remaining sliver of lines is revealed and the ranges join. When the merged
windows belonged to two hunks, the lower hunk's header SHALL be hidden and the two SHALL read as one
continuous block, as converging expansion does today. Merging SHALL be a property of the window
model — applied after any operation that changes windows (growth, default computation) — so the
window list is always sorted, disjoint, and separated by gaps of at least the threshold.

#### Scenario: Converging growth merges two windows

- **WHEN** growing reveals all but fewer-than-threshold lines of the gap between two windows
- **THEN** the remaining lines are revealed, the windows merge, no affordance remains between them,
  and a lower hunk's header is hidden

#### Scenario: A comment window near a hunk merges at creation

- **WHEN** a file is opened with a comment anchored just past a hunk's edge, so the default comment
  window would sit within the threshold of the hunk window
- **THEN** one merged window renders the hunk and the comment's context contiguously

### Requirement: Window state is in-session view state

Grown windows SHALL persist while the review remains open: leaving a file and returning to it SHALL
show the same windows, including growth. On a diff refresh, every file's windows SHALL collapse back
to their default sizes, recomputed from the refreshed hunks and comment placements.

#### Scenario: Growth survives switching files

- **WHEN** the reviewer grows a window, opens another file, and returns
- **THEN** the grown window renders at its grown size

#### Scenario: Refresh collapses windows to defaults

- **WHEN** the reviewer triggers a diff refresh after growing windows
- **THEN** every file's windows return to their default sizes, derived from the refreshed diff and
  the reconciled comment placements

### Requirement: Expansion affordances at hidden-line gaps

The diff viewer SHALL render a grow affordance at each window edge that faces hidden lines: above the first window, at both edges of each between-window gap, and below the last window. The affordance is active when hidden lines exist at that position and disabled when there are none, so the control stays present and predictable rather than appearing and disappearing.

#### Scenario: Hidden lines exist above the first window
- **WHEN** a file's first window begins below line 1 of the new file
- **THEN** a grow affordance is shown above that window offering to reveal earlier lines

#### Scenario: Hidden lines exist between two windows
- **WHEN** two adjacent windows are separated by hidden lines
- **THEN** grow affordances are shown in the gap between them

#### Scenario: Hidden lines exist below the last window
- **WHEN** the file continues beyond the last line of the last window
- **THEN** a grow affordance is shown below that window offering to reveal later lines

#### Scenario: No hidden lines at a file boundary
- **WHEN** the first window starts at line 1, or the last window ends at the final line of the file
- **THEN** the affordance for that boundary is still shown but rendered in a disabled state, so the control stays anchored at the top or bottom of the window even though it cannot reveal further lines

### Requirement: Incremental context expansion

Activating a grow affordance SHALL extend that window by a fixed number of previously hidden lines (the growth step, default 20) into the gap, inserted in correct file order with correct old and new line numbers, without altering other windows. Repeated activation SHALL continue until the gap is exhausted. Growing SHALL be reflected in the file's window state, not only in the rendered output, so a re-render from state reproduces the grown view.

When a gap closes, the outcome follows the window merge rule: a gap to another window merges the two windows; a gap at a file boundary leaves the affordance visible but disabled once exhausted. A disabled affordance SHALL ignore activation.

#### Scenario: Grow a window downward
- **WHEN** the reviewer activates a downward grow affordance and the gap holds more lines than one step
- **THEN** the next step of hidden lines is revealed at the window's bottom edge, and the affordance remains for further growth

#### Scenario: Grow a window upward
- **WHEN** the reviewer activates an upward grow affordance and the gap holds more lines than one step
- **THEN** the previous step of hidden lines is revealed at the window's top edge, and the affordance remains for further growth

#### Scenario: Gap smaller than a full step
- **WHEN** the reviewer activates a grow affordance and the remaining gap holds fewer lines than one step
- **THEN** all remaining hidden lines are revealed, and the gap resolves to its terminal state: windows merged, or a disabled affordance at a file boundary

#### Scenario: File-boundary gap fully revealed
- **WHEN** growth reveals the last hidden line above the first window or below the last window
- **THEN** the affordance remains in place but disabled, anchored at the window's edge, and ignores further activation

#### Scenario: A grown window re-renders identically from state
- **WHEN** a file region is re-rendered (for example after a comment mutation) following growth
- **THEN** the grown window renders at its grown extent, because growth lives in the window state rather than only in the previously rendered output

### Requirement: Full file content available at base and head revisions

The system SHALL provide the full content of a changed file at both the review's target (base) and source (head) revisions, so that unchanged lines outside the diff can be retrieved. Requesting content for a path or revision that does not exist SHALL fail without affecting the rest of the review.

#### Scenario: Retrieve context lines for an existing file
- **WHEN** the viewer requests a range of lines for a changed file that exists at the head revision
- **THEN** the system returns those lines from the head revision's full file content

#### Scenario: Requested file does not exist at the revision
- **WHEN** the viewer requests lines for a path that does not exist at the requested revision
- **THEN** the request reports an error and no expansion is rendered, while the rest of the diff remains usable

### Requirement: Revealed context preserves diff behaviour

Lines revealed by window growth SHALL behave as ordinary unchanged context lines: they display old and new line numbers, the new-line gutter remains clickable to add a comment, and any existing comment anchored to a revealed line's new-file line number SHALL appear once that line is shown.

A comment added to a revealed line SHALL be anchored to that line exactly as a comment added to a line already present in the diff: it SHALL render against its line immediately, SHALL NOT be treated as outdated or rendered untethered, and on later renders SHALL be visible via its own default window when its line falls outside the hunk windows.

#### Scenario: Comment on a revealed line
- **WHEN** a comment is anchored to a new-file line number that was hidden and is then revealed by growth
- **THEN** the comment thread is displayed against that line after it is revealed

#### Scenario: Add a comment on a revealed line
- **WHEN** the reviewer clicks the new-line gutter of a revealed context line
- **THEN** the add-comment interaction opens for that line number exactly as it does for lines originally in the diff

#### Scenario: A comment added to a revealed line stays anchored and visible

- **WHEN** the reviewer adds a comment to a revealed line, leaves the file, and returns
- **THEN** the comment renders against its line inside a window (a default comment window when the line is outside every hunk window)
- **AND** it is not marked outdated and does not render in the untethered block

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

### Requirement: Pure renames are listed with an explanatory placeholder

The diff SHALL be computed with rename detection forced on (git `-M`), so a file move with no content change is reported as a rename rather than as an unrelated delete-plus-add pair, regardless of the user's `diff.renames` configuration. A file whose diff carries a `rename from <path>` marker SHALL be flagged as renamed during diff parsing, recording its pre-rename path. A renamed file SHALL appear in the file navigation. Selecting a pure rename — one flagged as renamed and carrying no hunks — SHALL show an explanatory placeholder naming the old path ("renamed from `<old>`, no changes") in place of a diff body, with no hunks rendered, so the empty body reads as expected rather than broken. A rename that also carries content edits SHALL render its hunks normally and SHALL NOT show the placeholder.

#### Scenario: Diff forces rename detection

- **WHEN** a file is moved with no content change and the repository has `diff.renames` disabled
- **THEN** the diff still reports it as a rename, carrying `rename from`/`rename to` markers rather than a separate delete and add

#### Scenario: Diff marks a file as a pure rename

- **WHEN** a changed file's diff section contains a `rename from <old>` marker and no hunks
- **THEN** that file is flagged as renamed in the parsed diff and its old path is recorded

#### Scenario: Renamed file appears in navigation

- **WHEN** the file list is rendered and one changed file is a pure rename
- **THEN** the renamed file is listed alongside other changed files

#### Scenario: Selecting a pure rename

- **WHEN** the reviewer selects a file flagged as renamed that carries no hunks
- **THEN** an explanatory placeholder reading "renamed from `<old>`, no changes" is shown in place of a diff body, with no hunks rendered

#### Scenario: Rename with edits renders normally

- **WHEN** the reviewer selects a file flagged as renamed that also carries hunks
- **THEN** its hunks render as for any other changed file and no rename placeholder is shown

### Requirement: Open a changed file in the preferred application

The filename line for a changed file SHALL offer a "browse" action that opens the file at its working-tree path in the operating system's preferred application via `xdg-open`. Activating it SHALL NOT alter the review state, and a failure to open SHALL NOT disrupt the review.

#### Scenario: Browse a changed file
- **WHEN** the reviewer activates the browse action for a changed file
- **THEN** the file at its working-tree path is opened with `xdg-open` in the OS-preferred application

#### Scenario: Browse target cannot be opened
- **WHEN** activating the browse action fails (for example the path is absent or no opener is available)
- **THEN** the failure is reported without changing review state and the rest of the review remains usable

