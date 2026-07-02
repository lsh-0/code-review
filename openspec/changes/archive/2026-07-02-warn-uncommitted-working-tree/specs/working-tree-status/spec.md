## ADDED Requirements

### Requirement: Surface uncommitted changes with a page-wide banner

The system SHALL display a page-wide banner when the working-tree status reports any tracked file
modified or deleted, stating that uncommitted changes were detected and counting the modified and the
deleted files. When the tree is clean the banner SHALL NOT be shown. The banner SHALL refresh whenever
the review is refreshed, since a new commit can change what is uncommitted.

#### Scenario: Dirty tree shows the banner with counts

- **WHEN** the working-tree status reports modified or deleted tracked files
- **THEN** a page-wide banner appears reporting the count of modified and deleted files

#### Scenario: Clean tree shows no banner

- **WHEN** the working-tree status reports no modified and no deleted files
- **THEN** no page-wide banner is shown

### Requirement: Warn per file and offer the configured diff tool

The system SHALL display a per-file warning banner when the file being viewed has uncommitted
working-tree changes, indicating that the file contains changes not reflected in the diff. The banner
SHALL offer a control that opens the file in the reviewer's configured diff tool via `git difftool`,
which honours their `diff.tool`. A file with no uncommitted changes, and the review overview, SHALL show
no per-file banner.

#### Scenario: Viewing a dirty file warns and links to the diff tool

- **WHEN** the reviewer views a file that has uncommitted working-tree changes
- **THEN** a per-file banner appears warning that the file contains uncommitted changes
- **AND** the banner offers a control to open the file in the configured diff tool

#### Scenario: Viewing a clean file shows no per-file banner

- **WHEN** the reviewer views a file with no uncommitted working-tree changes
- **THEN** no per-file banner is shown

### Requirement: Update the banners live when the working tree changes

The system SHALL update the uncommitted-change banners on its own shortly after the
working tree changes on disk, without the reviewer triggering a manual refresh. The
system SHALL detect the change by periodically re-running the working-tree query
(not by monitoring individual files) and SHALL re-render the banners when the
reported status differs from the previous poll. A change that leaves the reported
status unchanged SHALL cause no banner update.

#### Scenario: Editing a tracked file raises the banner without a manual refresh

- **WHEN** a tracked file is modified on disk while the review is open and no manual
  refresh is triggered
- **THEN** the page-wide banner appears (or updates its counts) within one poll
  interval

#### Scenario: Reverting the last change clears the banner without a manual refresh

- **WHEN** the working tree returns to clean on disk while the review is open and no
  manual refresh is triggered
- **THEN** the page-wide banner disappears within one poll interval

#### Scenario: A change that does not alter the reported status causes no update

- **WHEN** a poll reports the same modified and deleted files as the previous poll
- **THEN** no banner re-render is triggered
