## MODIFIED Requirements

### Requirement: Surface uncommitted changes with a page-wide banner

The system SHALL display a page-wide banner when the working-tree status reports any tracked file
modified or deleted, stating that uncommitted changes were detected. The banner SHALL NOT report how
many files are modified or deleted: it is a binary condition indicator, and the reviewer learns which
files are affected from the per-file banner and the file list. When the tree is clean the banner SHALL
NOT be shown. The banner SHALL refresh whenever the review is refreshed, since a new commit can change
whether anything is uncommitted.

#### Scenario: Dirty tree shows the banner

- **WHEN** the working-tree status reports modified or deleted tracked files
- **THEN** a page-wide banner appears reporting that uncommitted changes were detected

#### Scenario: The banner does not vary with the number of affected files

- **WHEN** the working-tree status reports one modified file, and later several modified and deleted
  files
- **THEN** the banner's message is the same in both cases

#### Scenario: Clean tree shows no banner

- **WHEN** the working-tree status reports no modified and no deleted files
- **THEN** no page-wide banner is shown

### Requirement: Update the banners live when the working tree changes

The system SHALL update the uncommitted-change banners on its own shortly after the
working tree changes on disk, without the reviewer triggering a manual refresh. The
system SHALL detect the change by periodically re-running the working-tree query
(not by monitoring individual files) and SHALL re-render the banners when the
reported status differs from the previous poll. A change that leaves the reported
status unchanged SHALL cause no banner update.

Once the page-wide banner is raised, the system SHALL stop reporting further dirty states while it
remains raised, since a dirty tree is a steady condition the reviewer can already see rather than news.
The system SHALL continue to evaluate whether the tree has returned to clean, and SHALL clear the
banner and resume normal reporting when it has. Because the banner reports no counts, suspending in
this way SHALL NOT cause it to display anything inaccurate.

The per-file banner SHALL continue to reflect the file being viewed, since it is rendered on selection
from the last known status rather than driven by the poll.

#### Scenario: Editing a tracked file raises the banner without a manual refresh

- **WHEN** a tracked file is modified on disk while the review is open and no manual
  refresh is triggered
- **THEN** the page-wide banner appears within one poll interval

#### Scenario: Further edits while the banner is raised do not re-report

- **WHEN** the banner is already raised and additional tracked files are modified
- **THEN** no further dirty-state report is made while the banner remains raised
- **AND** the banner's message remains accurate, since it reports no counts

#### Scenario: Reverting the last change clears the banner without a manual refresh

- **WHEN** the working tree returns to clean on disk while the review is open and no
  manual refresh is triggered
- **THEN** the page-wide banner disappears within one poll interval

#### Scenario: Returning to clean resumes normal reporting

- **WHEN** the tree has gone dirty, then returns to clean, then goes dirty again
- **THEN** the banner is raised again for the second dirty state

#### Scenario: A change that does not alter the reported status causes no update

- **WHEN** a poll reports the same modified and deleted files as the previous poll
- **THEN** no banner re-render is triggered
