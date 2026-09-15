## ADDED Requirements

### Requirement: Anchor history is private to the backend

Anchor histories SHALL exist only within the backend model and its persisted state. Every consuming
surface — the GUI over the bridge and the agent CLI alike — SHALL receive a comment's placement as
**derived** fields computed by the backend: the current new-side line number (0 when the comment has
no placement), whether the comment is outdated, and the most-recent good context for rendering an
untethered comment. Surfaces SHALL NOT receive raw anchor entries and SHALL NOT re-derive placement
or outdatedness themselves. Creating a line comment SHALL require only the file, content, and line
number; the backend SHALL capture the anchor's context window itself from the file's content at the
head revision.

#### Scenario: The GUI receives derived placement, not anchors

- **WHEN** the frontend fetches a file's comments over the bridge
- **THEN** each comment carries its current line number, outdated flag, and last good context as
  fields computed by the backend
- **AND** no anchor history entries cross the bridge

#### Scenario: Creating a comment sends only file, content, and line

- **WHEN** the reviewer saves a comment on a line
- **THEN** the frontend sends the file path, the content, and the line number
- **AND** the backend captures the context window for the first anchor from the file's head-revision
  content

#### Scenario: One derivation serves the GUI and the CLI

- **WHEN** the GUI and the agent CLI each report the same comment
- **THEN** both present the same backend-derived line number and outdated flag

## MODIFIED Requirements

### Requirement: Comments carry a history of blob-keyed anchors

A comment SHALL anchor to its code through an ordered history of anchors rather than a single line number. Each anchor SHALL record the file's git blob SHA it was computed against, a new-side line number, a captured window of surrounding line contents, and the **offset** of the anchored line within that window (its index in the captured lines). The first anchor (created when the comment is made) SHALL be a good anchor: it SHALL carry a real line number and a non-empty captured context.

The captured window SHALL be drawn by the backend from the file's content at the head revision — the anchored line and its neighbours on each side, clipped only at the bounds of the file. It SHALL NOT be limited to the lines present in the diff, and SHALL NOT include old-side (removed) lines. A comment made against a line revealed by window growth SHALL therefore capture a good first anchor exactly as one made against a line already present in the diff.

The anchored line SHALL NOT be assumed to be the centre of the captured window. A comment near the start or end of the file has fewer neighbours on that side, so the offset SHALL be recorded explicitly rather than derived from the window length.

An anchor with an empty captured context SHALL be an **adrift** anchor and SHALL carry no meaningful line number or offset.

#### Scenario: Creating a comment captures a good first anchor

- **WHEN** a reviewer adds a comment against a line of the diff
- **THEN** the comment's first anchor records the file's current blob SHA, the new-side line number, a window of surrounding line contents, and the anchored line's offset within that window
- **AND** that first anchor is a good anchor (non-empty context)

#### Scenario: Commenting on a revealed line captures a good first anchor

- **WHEN** a reviewer grows a window and adds a comment to one of the revealed lines
- **THEN** the comment's first anchor captures a non-empty context window drawn from the file's content around that line
- **AND** the comment is anchored to that line and is not outdated

#### Scenario: Context window is wider than a single line

- **WHEN** a comment's first anchor is captured
- **THEN** the captured context includes the anchored line and neighbouring lines on each side, drawn from the file's content at the head revision

#### Scenario: Captured window excludes removed lines

- **WHEN** a comment is anchored to a line whose neighbours in the diff include lines removed from the old side
- **THEN** the captured window contains only new-side line contents, so it matches the same line source the reconciler searches

#### Scenario: Offset locates the anchored line in a clipped window

- **WHEN** a comment is anchored to a line within the first or last few lines of the file, so the window is clipped on one side
- **THEN** the recorded offset is the anchored line's actual index within the captured window, which is not the window's centre

### Requirement: New content is re-anchored by exact-then-fuzzy matching against the last good context

When a comment's file presents a blob SHA not already in the comment's history, the system SHALL re-anchor by matching the comment's last good context (the most recent anchor that has a non-empty context) against the file's full content at the head revision — every line of the file, not only the lines present in the recomputed diff. The system SHALL first attempt an exact match; if no exact match is found it SHALL attempt a fuzzy match and accept it only if its similarity meets a confidence threshold. The fuzzy search SHALL target the last good context only and SHALL NOT fan out over older anchors' contexts. On a successful match the system SHALL append a good anchor for the new blob SHA; if neither exact nor fuzzy matching succeeds the system SHALL append an adrift anchor carrying only the new blob SHA.

The last good context SHALL serve only as the search key that locates the line in the new content. On a successful match the appended good anchor SHALL capture a **fresh** context window (and its own offset) from the new content at the matched position; it SHALL NOT carry the search context forward. This keeps each anchor's context a witness of the blob it belongs to, so the next reconciliation matches against the latest successful capture rather than the original.

A file whose content cannot be read at the head revision SHALL be skipped for that reconciliation, leaving its comments untouched, rather than having its comments marked adrift on a transient failure.

#### Scenario: A re-anchor captures a fresh window from the new content

- **WHEN** a comment re-anchors onto new content in which some neighbouring lines around the anchored line have changed
- **THEN** the appended anchor's captured context reflects the new content at the matched position, not the prior search context
- **AND** a subsequent reconciliation matches against that fresh context

#### Scenario: Shifted line re-anchors by exact match

- **WHEN** a comment's file changes to new content in which the comment's last good context still appears verbatim at a shifted position
- **THEN** the system appends a good anchor at the matched line for the new blob SHA
- **AND** the comment renders at the new line

#### Scenario: Similar content re-anchors by fuzzy match above threshold

- **WHEN** a comment's last good context has no exact match but a candidate window meets the confidence threshold
- **THEN** the system appends a good anchor at that candidate for the new blob SHA

#### Scenario: Content outside the diff re-anchors against the full file

- **WHEN** a comment's last good context resides in an unchanged region of the file that is absent from the recomputed diff
- **THEN** the system matches it against the file's full content and appends a good anchor at that line
- **AND** the comment stays anchored rather than becoming adrift

#### Scenario: Lost content becomes adrift

- **WHEN** a comment's last good context matches nowhere in the file's full content, neither exactly nor above the confidence threshold
- **THEN** the system appends an adrift anchor carrying only the new blob SHA
- **AND** the comment becomes outdated

#### Scenario: Unreadable file leaves comments untouched

- **WHEN** a commented file's content cannot be read at the head revision during a reconciliation
- **THEN** that file's comments are left exactly as they were and no adrift anchor is appended

### Requirement: Outdated comments render untethered with a warning indicator

The system SHALL render an outdated comment detached from any window, positioned at the top of its file's view, displaying its most-recent good context as a read-only pseudo-hunk marked with a warning (yellow) border. An untethered outdated comment SHALL NOT offer a grow affordance. When a file also has top-level (anchorless) comments, the untethered outdated comments SHALL render **below** the file's top-level comments and **above** the file's code, giving a top-of-file order of top-level comments, then untethered comments, then code.

A comment that is **not** outdated but whose anchored line cannot be shown — for example a line number beyond the end of the file — SHALL fall back to the same untethered presentation rather than disappearing from the view. This fallback SHALL be a rendering decision only: it SHALL NOT mark the comment adrift, so the comment re-places itself against its line as soon as that line can be shown again.

#### Scenario: Outdated comment shows its captured context at the top of the file

- **WHEN** a file with an outdated comment is rendered
- **THEN** the outdated comment appears at the top of the file's view as a read-only pseudo-hunk built from its most-recent good context
- **AND** the pseudo-hunk has a warning border and no grow control

#### Scenario: A non-outdated comment renders against its line as before

- **WHEN** a file's comment has a good most-recent anchor
- **THEN** the comment renders against its anchored line within a window, not at the top of the file

#### Scenario: An anchored comment whose line cannot be shown falls back to untethered

- **WHEN** a comment has a good anchor but its line cannot be shown in any window
- **THEN** the comment renders in the untethered block at the top of the file
- **AND** its anchor is unchanged, so it renders against its line again once that line can be shown

#### Scenario: Untethered comments sit below top-level comments

- **WHEN** a file has both a top-level (anchorless) comment and an outdated comment
- **THEN** the outdated comment's untethered block renders below the top-level comment
- **AND** above the file's code
