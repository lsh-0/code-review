## ADDED Requirements

### Requirement: The file list selects an ordered set of files

The file list SHALL maintain an ordered selection of one or more files. A plain
click on a file item SHALL select exactly that file, replacing any previous
selection. A ctrl-click or cmd-click on a file item SHALL toggle that file in or out
of the current selection, leaving the other selected files unchanged. The selection
SHALL always contain at least one file while a file (not the overview) is shown: a
ctrl/cmd-click that would remove the last remaining file SHALL be a no-op.

#### Scenario: A plain click selects a single file

- **WHEN** the reviewer plain-clicks a file item while one or more files are selected
- **THEN** the selection becomes exactly that one file
- **AND** the diff pane shows only that file's diff

#### Scenario: A ctrl/cmd-click adds a file to the selection

- **WHEN** the reviewer ctrl-clicks or cmd-clicks a file item that is not selected
- **THEN** that file is appended to the selection
- **AND** the previously selected files remain selected

#### Scenario: A ctrl/cmd-click removes an already-selected file

- **WHEN** the reviewer ctrl-clicks or cmd-clicks a file item that is already
  selected, and it is not the only selected file
- **THEN** that file is removed from the selection
- **AND** the other selected files remain selected

#### Scenario: The selection never empties

- **WHEN** the reviewer ctrl-clicks or cmd-clicks the only selected file
- **THEN** the selection is unchanged and that file remains shown

### Requirement: Every selected file is marked active in the file list

The file list SHALL mark every file in the current selection as active, not just
one. Selecting the overview SHALL clear the file selection's active marks and mark
the overview entry active instead.

#### Scenario: Multiple selected files are all highlighted

- **WHEN** two or more files are selected
- **THEN** each of their file-list items carries the active state
- **AND** no other file item does

### Requirement: The diff pane renders the selection as one stacked view

When more than one file is selected, the diff pane SHALL render the selected files
as one stacked view, in selection order. Each file SHALL be rendered as a section
that begins with that file's filename header and is followed by that file's full
diff — its hunks, expand affordances, and embedded comment threads — exactly as the
single-file view renders one file. A single-file selection SHALL render identically
to the previous single-file view.

#### Scenario: Two selected files render as two labelled sections

- **WHEN** two files are selected
- **THEN** the diff pane contains one section per file, in selection order
- **AND** each section begins with a header showing that file's path
- **AND** each section contains that file's hunks with expand affordances and any
  comment threads

#### Scenario: The single-file view is unchanged

- **WHEN** exactly one file is selected
- **THEN** the diff pane renders that file's diff as it did before this capability
  existed

### Requirement: The per-file label moves into each stacked section

The filename label SHALL appear in each stacked section's header so every section is
labelled. The single sticky pane header SHALL summarise the selection — the file's
path when one file is selected, and a count ("N files") when more than one is
selected.

#### Scenario: The sticky header summarises a multi-file selection

- **WHEN** more than one file is selected
- **THEN** the sticky pane header shows a count of the selected files
- **AND** each stacked section's header shows its own file's path

### Requirement: Per-file controls and mutations act on the right file

Per-file controls and updates SHALL act on the file whose section they belong to,
even when several files are shown and a line number recurs across files. This covers
the browse control, comment mutations, and expand affordances. A comment mutation on
one file's line SHALL patch only that file's section.

#### Scenario: A comment mutation patches only its own file's section

- **WHEN** a comment is added, edited, resolved, or deleted on a line of one
  selected file while several files are shown
- **THEN** only that file's section is updated
- **AND** a line with the same number in another selected file's section is left
  untouched
