## ADDED Requirements

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

## MODIFIED Requirements

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
