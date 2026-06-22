## ADDED Requirements

### Requirement: Comments carry a history of blob-keyed anchors

A comment SHALL anchor to its code through an ordered history of anchors rather than a single line number. Each anchor SHALL record the file's git blob SHA it was computed against, a new-side line number, a captured window of surrounding line contents, and the **offset** of the anchored line within that window (its index in the captured lines). The first anchor (created when the comment is made) SHALL be a good anchor: it SHALL carry a real line number and a non-empty captured context.

The anchored line SHALL NOT be assumed to be the centre of the captured window. Because the window is drawn from within a single hunk, a comment near the start or end of its hunk has fewer neighbours on that side, so the offset SHALL be recorded explicitly rather than derived from the window length.

An anchor with an empty captured context SHALL be an **adrift** anchor and SHALL carry no meaningful line number or offset.

#### Scenario: Creating a comment captures a good first anchor

- **WHEN** a reviewer adds a comment against a line of the diff
- **THEN** the comment's first anchor records the file's current blob SHA, the new-side line number, a window of surrounding line contents, and the anchored line's offset within that window
- **AND** that first anchor is a good anchor (non-empty context)

#### Scenario: Context window is wider than a single line

- **WHEN** a comment's first anchor is captured
- **THEN** the captured context includes the anchored line and neighbouring lines on each side, drawn from the hunk the comment sits in

#### Scenario: Offset locates the anchored line in a clipped window

- **WHEN** a comment is anchored to a line at the very top or bottom of its hunk, so the window is clipped on one side
- **THEN** the recorded offset is the anchored line's actual index within the captured window, which is not the window's centre

### Requirement: A comment's current placement and outdated state are derived from its anchors

The system SHALL derive a comment's current line number from its most-recent anchor, and SHALL derive whether a comment is **outdated** from whether its most-recent anchor is adrift. These derived properties SHALL be orthogonal to the comment's `Status` (active/resolved/ignored): a comment MAY be simultaneously outdated and in any status. The system SHALL NOT store an `outdated` flag or a standalone current-line field as a separate source of truth.

#### Scenario: Current line comes from the most-recent anchor

- **WHEN** a comment's most-recent anchor is a good anchor
- **THEN** the comment renders at that anchor's line number

#### Scenario: Outdated is derived, not a status

- **WHEN** a comment's most-recent anchor is adrift
- **THEN** the comment is outdated
- **AND** its `Status` (active, resolved, or ignored) is unchanged by becoming outdated

### Requirement: Reconciling a comment reuses an existing anchor before matching

When reconciling a comment against its file's current blob SHA, the system SHALL first attempt reuse: if the current blob SHA already appears anywhere in the comment's anchor history, the system SHALL reuse that anchor's placement and context directly, without running any context matching. Reuse of a good anchor SHALL recover a previously adrift comment; reuse of an adrift anchor SHALL leave the comment adrift. If the current blob SHA equals the most-recent anchor's blob SHA, the file is unchanged for this comment and no new anchor SHALL be appended.

#### Scenario: Unchanged file content does nothing

- **WHEN** a comment is reconciled and its file's current blob SHA equals its most-recent anchor's blob SHA
- **THEN** no new anchor is appended and the comment's placement is unchanged

#### Scenario: Reverted content recovers an adrift comment by reuse

- **WHEN** a comment is adrift and the file's content reverts to a blob SHA already present in the comment's anchor history with a good anchor
- **THEN** the system reuses that good anchor without running context matching
- **AND** the comment is no longer outdated

### Requirement: New content is re-anchored by exact-then-fuzzy matching against the last good context

When a comment's file presents a blob SHA not already in the comment's history, the system SHALL re-anchor by matching the comment's last good context (the most recent anchor that has a non-empty context) against the lines present in the recomputed diff for that file. The system SHALL first attempt an exact match; if no exact match is found it SHALL attempt a fuzzy match and accept it only if its similarity meets a confidence threshold. The fuzzy search SHALL target the last good context only and SHALL NOT fan out over older anchors' contexts. On a successful match the system SHALL append a good anchor for the new blob SHA; if neither exact nor fuzzy matching succeeds the system SHALL append an adrift anchor carrying only the new blob SHA.

The last good context SHALL serve only as the search key that locates the line in the new content. On a successful match the appended good anchor SHALL capture a **fresh** context window (and its own offset) from the new content at the matched position; it SHALL NOT carry the search context forward. This keeps each anchor's context a witness of the blob it belongs to, so the next reconciliation matches against the latest successful capture rather than the original.

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

#### Scenario: Lost content becomes adrift

- **WHEN** a comment's last good context has no exact match and no candidate meets the confidence threshold
- **THEN** the system appends an adrift anchor carrying only the new blob SHA
- **AND** the comment becomes outdated

#### Scenario: Content outside the diff cannot be re-anchored

- **WHEN** a comment's last good context now resides only in an unchanged region of the file that is absent from the recomputed diff
- **THEN** the comment becomes adrift rather than being placed against a line not present in the diff

### Requirement: Outdated comments render untethered with a warning indicator

The system SHALL render an outdated comment detached from any live hunk, positioned at the top of its file's view, displaying its most-recent good context as a read-only pseudo-hunk marked with a warning (yellow) border. An untethered outdated comment SHALL NOT offer a line-expand affordance.

#### Scenario: Outdated comment shows its captured context at the top of the file

- **WHEN** a file with an outdated comment is rendered
- **THEN** the outdated comment appears at the top of the file's view as a read-only pseudo-hunk built from its most-recent good context
- **AND** the pseudo-hunk has a warning border and no line-expand control

#### Scenario: A non-outdated comment renders against its line as before

- **WHEN** a file's comment has a good most-recent anchor
- **THEN** the comment renders against its anchored line in the live diff, not at the top of the file

### Requirement: Deleting an outdated comment removes its captured context

The system SHALL, when a reviewer deletes an outdated comment, remove the comment together with its captured anchor context. No orphaned context SHALL remain after the comment is deleted.

#### Scenario: Deleting an outdated comment leaves nothing behind

- **WHEN** a reviewer deletes an outdated comment
- **THEN** the comment and its captured context are both removed
- **AND** the untethered pseudo-hunk for that comment is no longer rendered

### Requirement: Legacy comments upgrade to anchors on load

The system SHALL load review state written before this change without a separate migration step. A comment that has a line number and captured context but no anchor history SHALL be upgraded in place to a single first anchor whose blob SHA is empty, marking it as a legacy anchor awaiting backfill. On the first reconciliation, a legacy first anchor SHALL adopt the file's current blob SHA as its baseline without being treated as adrift.

#### Scenario: Legacy comment loads as a single legacy anchor

- **WHEN** review state containing a comment with a line number and context but no anchors is loaded
- **THEN** the comment is upgraded to a single first anchor with that line number and context and an empty blob SHA

#### Scenario: Legacy anchor adopts a baseline on first reconciliation

- **WHEN** a comment whose first anchor has an empty blob SHA is reconciled against its file's current blob SHA
- **THEN** the anchor adopts the current blob SHA as its baseline
- **AND** the comment is not made outdated by the adoption
