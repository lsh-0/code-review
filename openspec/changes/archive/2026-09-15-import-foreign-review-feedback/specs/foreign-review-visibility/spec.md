## ADDED Requirements

### Requirement: Feedback sources are togglable, with local feedback among them

The GUI SHALL present the review's feedback sources as a list of toggles, one per
source, each controlling whether that source's comments are shown.

Local feedback SHALL be the first entry in that list and SHALL toggle by the same
mechanism as any imported source. The reviewer therefore has one uniform control
rather than a special case for the tool's own comments.

The control SHALL NOT be shown when the review has no imported sources. With
nothing imported, the list would hold a single toggle whose only effect is to hide
every comment in the review, which is a trap rather than a choice. The control
appears with the first import.

Every source SHALL be visible by default, so opening a review with imports shows
all its feedback without first configuring anything.

#### Scenario: The list shows local feedback first

- **WHEN** a review with one imported source is opened
- **THEN** the source toggles list two entries
- **AND** the first is local feedback
- **AND** the second is the imported source, labelled with its display label

#### Scenario: A review with no imports shows no control

- **WHEN** a review with no imported sources is opened
- **THEN** the source-toggle control is not shown

#### Scenario: Importing reveals the control

- **WHEN** a document is imported into a review that had no sources
- **THEN** the source-toggle control appears, listing local feedback and the imported source

#### Scenario: Every source starts visible

- **WHEN** a review is opened
- **THEN** every source in the list is toggled on

### Requirement: Hiding a source hides its comments everywhere they render

Hiding a source SHALL remove its comments from every rendered surface: the
line-anchored threads in the diff pane, the untethered block of outdated
comments, and the review overview's per-file and review-level sections.

Hiding a source SHALL also remove its comments from the file-list status pills and
from any rendered comment counts, so the file list agrees with what the diff pane
shows. A file whose only comments belong to hidden sources SHALL show no status
pill.

#### Scenario: Hidden comments leave the diff pane

- **WHEN** a source is toggled off while a file with comments from that source is displayed
- **THEN** those comments are no longer rendered against their lines

#### Scenario: Hidden comments leave the untethered block

- **WHEN** a source with untethered comments is toggled off
- **THEN** those comments are no longer rendered in the untethered block
- **AND** the block itself is not rendered if no visible comment remains untethered

#### Scenario: Hidden comments leave the overview

- **WHEN** a source is toggled off and the overview is displayed
- **THEN** neither its file-anchored nor its review-level comments are shown
- **AND** a file whose only comments came from that source is not listed

#### Scenario: File pills reflect only visible sources

- **WHEN** a file's only comments belong to a source that is toggled off
- **THEN** that file shows no comment-status pill

#### Scenario: A mixed file's pill reflects the visible subset

- **WHEN** a file has an active comment from a hidden source and a resolved comment from a visible source
- **THEN** its pill shows the resolved status

### Requirement: Hiding a source hides its local replies with it

A local reply to an imported comment belongs to the thread it answers. Hiding the
source of that thread SHALL hide the reply with it, because a reply rendered
without the comment it answers has no context.

Hiding local feedback SHALL hide local root comments and their threads. It SHALL
NOT hide a local reply whose root belongs to a visible imported source, because
that reply is part of a visible thread.

#### Scenario: A reply to a hidden imported comment is hidden

- **WHEN** an imported comment has a local reply, and that comment's source is toggled off
- **THEN** neither the imported comment nor its local reply is rendered

#### Scenario: A reply to a visible imported comment survives hiding local feedback

- **WHEN** local feedback is toggled off and an imported source is visible
- **THEN** an imported comment with a local reply still renders with its reply
- **AND** local root comments and their threads are not rendered

### Requirement: Visibility is view state and is not persisted

Source visibility SHALL be held as view state for the session only. It SHALL NOT
be written to the review state file, and SHALL NOT be readable by the CLI, so
hiding a source in the GUI never changes what an agent sees.

Visibility SHALL reset to all-visible when the GUI is restarted, matching how the
file-list grouping already resets.

#### Scenario: Toggling does not write the state file

- **WHEN** a source is toggled off
- **THEN** the review state file is not written

#### Scenario: Visibility resets on restart

- **WHEN** a source is toggled off and the GUI is restarted
- **THEN** every source is visible again

#### Scenario: A hidden source is still visible to the CLI

- **WHEN** a source is toggled off in the GUI and the CLI lists active comments
- **THEN** that source's active comments are listed
