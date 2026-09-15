## MODIFIED Requirements

### Requirement: Comments carry a history of blob-keyed anchors

A comment SHALL anchor to its code through an ordered history of anchors rather than a single line number. Each anchor SHALL record the file's git blob SHA it was computed against, a new-side line number, a captured window of surrounding line contents, and the **offset** of the anchored line within that window (its index in the captured lines). The first anchor SHALL be a good anchor — a real line number and a non-empty captured context — whenever the anchored line can be located, whether the comment was created by a reviewer or brought in by an import.

An imported comment's first anchor SHALL be built at import time from the current diff at the line the import states, using the same captured-window size as a reviewer-created anchor. An imported comment is therefore indistinguishable to the reconciler from one created locally at that line, and needs no separate placement path.

Where the anchored line cannot be located — an imported line absent from the current diff, an imported position that addresses no new-side line — the first anchor SHALL be adrift, carrying only the current blob SHA. This is the one case where a first anchor is not good; a reviewer-created first anchor is always good, since the reviewer clicked a line that exists.

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

#### Scenario: An imported comment gets a good first anchor at its stated line

- **WHEN** a comment is imported at a line present in the current diff
- **THEN** its first anchor records the file's current blob SHA, that line number, a captured window of the same size a reviewer-created anchor would capture, and the anchored line's offset
- **AND** that first anchor is a good anchor

#### Scenario: An imported comment at an unlocatable line starts adrift

- **WHEN** a comment is imported at a line absent from the current diff, or with a position that addresses no new-side line
- **THEN** its first anchor is adrift, carrying only the current blob SHA
- **AND** the comment is outdated from the moment it is imported

#### Scenario: An imported comment reconciles like a local one

- **WHEN** an imported comment with a good first anchor is reconciled after its file's content changes
- **THEN** it is re-anchored, recovered, or marked adrift by exactly the rules that apply to a locally created comment
