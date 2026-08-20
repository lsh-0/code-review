# text-zoom Specification

## Purpose
TBD - created by archiving change scale-comment-text-with-zoom. Update Purpose after archive.
## Requirements
### Requirement: Ctrl+zoom drives a single text scale for the review surface

The application SHALL provide its own text zoom, because the native WebKit zoom is
unreliable under Wails. **Ctrl+wheel**, **Ctrl+=** / **Ctrl++** and **Ctrl+-**
SHALL adjust a single zoom factor, and **Ctrl+0** SHALL reset it to 1.0. The
factor SHALL be clamped to the range 0.5–3.0 inclusive, and SHALL be exposed to
the stylesheet as one custom property on the root element so that every scaling
rule reads from the same value. The zoom factor is session state and SHALL NOT be
persisted across restarts.

This requirement codifies pre-existing behaviour, which is unchanged; it is
specified here because the scaling requirements below are defined relative to it.

#### Scenario: Ctrl+wheel and Ctrl+= change the scale

- **WHEN** the user Ctrl+wheels up, or presses Ctrl+= or Ctrl++
- **THEN** the zoom factor increases by one step and the scaled text grows
- **AND** Ctrl+wheel down or Ctrl+- decreases it by one step

#### Scenario: The scale is clamped at both ends

- **WHEN** the user keeps zooming out past the minimum, or in past the maximum
- **THEN** the factor stops at 0.5 and at 3.0 respectively rather than continuing

#### Scenario: Ctrl+0 resets the scale

- **WHEN** the user presses Ctrl+0 at any zoom factor
- **THEN** the factor returns to 1.0 and all scaled text returns to its base size

### Requirement: Comment text scales with the zoom factor

Comment threads SHALL scale with the same zoom factor as the diff code, so a
reviewer who zooms in to read the code reads the review conversation at a matching
size. Scaling SHALL cover the comment body text, the author, the status badge, and
the thread's action buttons, and SHALL preserve their existing size proportions
relative to one another — zoom changes the scale of the comment card, not its
internal typographic hierarchy.

Scaling SHALL apply in every context a thread renders in — line-anchored threads,
top-level (anchorless) file comments, and outdated (untethered) comments — without
each context needing its own rule.

#### Scenario: Comment body text grows with the code

- **WHEN** the user zooms in while a file with line-anchored comments is shown
- **THEN** the comment body text grows by the same factor as the diff code
- **AND** it shrinks by the same factor when the user zooms out

#### Scenario: A comment's parts keep their proportions

- **WHEN** the comment text is scaled to any zoom factor
- **THEN** the author, status badge, and action buttons scale with the body text
- **AND** each stays proportionally smaller than the body text, as at zoom 1.0

#### Scenario: Every thread context scales

- **WHEN** the user zooms with top-level comments and outdated comments on screen
- **THEN** the text of those threads scales exactly as a line-anchored thread's does

#### Scenario: Ctrl+0 returns comments to their base size

- **WHEN** the user has zoomed comment text and presses Ctrl+0
- **THEN** the comment text returns to the size it had before any zooming

### Requirement: Comment-entry controls scale with the comments they act on

The controls for adding and writing a comment SHALL scale with the comment text,
so no control sits at a fixed size beside text that has grown. This SHALL cover the
"Add comment" affordances rendered alongside threads (the top-level file control
and the overview's review-level control) and the comment compose/edit modal.

The modal SHALL scale as a whole — its title, its text area, and its action
buttons — since every modal in the application is a comment modal (add, edit,
reply) and a modal that grew only in parts would read as broken rather than
zoomed. The file-content header that labels the diff view is NOT part of this: it
labels the view rather than a comment, and SHALL remain fixed (see the chrome
requirement below).

The compose/edit modal SHALL also grow its width with the zoom factor, so that
text entered at a large zoom is not confined to a fixed narrow box. The modal
SHALL remain within the window at every zoom factor.

#### Scenario: Add-comment controls scale

- **WHEN** the user zooms in with a top-level "Add comment" control on screen
- **THEN** that control's label scales with the comment text beside it

#### Scenario: The compose box matches the reading size

- **WHEN** the user zooms in and then opens the comment modal to write a comment
- **THEN** the text they type appears at the scaled comment size

#### Scenario: The modal scales as a whole

- **WHEN** the user opens the comment modal at a zoom factor above 1.0
- **THEN** its title, text area, and action buttons are all scaled by that factor
- **AND** no part of the modal remains at its unzoomed size

#### Scenario: The modal stays inside the window when zoomed

- **WHEN** the modal is opened at the maximum zoom factor
- **THEN** the modal is wider than at zoom 1.0
- **AND** it does not exceed the window width

### Requirement: Application chrome does not scale

Zoom SHALL govern the review reading surface only. The application header, the
sidebar and its file list — including the per-file comment-count badge — the
toolbar controls, and the file-content header naming the file on display SHALL
keep their fixed sizes at every zoom factor, so that zooming in on the text under
review does not consume the window with navigation chrome.

#### Scenario: The file list keeps its size while the diff grows

- **WHEN** the user zooms in
- **THEN** the diff text and comment text grow
- **AND** the file-list entries, their comment-count badges, the application
  header, and the file-content header naming the current file stay at their
  original size

