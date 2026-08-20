# working-tree-status Specification

## Purpose
Answer "what has changed in the working tree" as a typed query distinct from the committed branch diff,
so the GUI can warn that what the reviewer sees may not reflect what is on disk, and so a refresh can
drop the "done" mark from any file a new commit has changed. Tracked modified and deleted files are
reported; untracked files are excluded, since the review concerns committed changes.

## Requirements
### Requirement: Query working-tree status distinctly from the branch diff

The system SHALL provide a working-tree status query that reports tracked files modified or deleted
relative to the working tree, as a typed result distinct from the committed branch-to-branch diff.
Untracked (new) files SHALL NOT be reported. The two queries SHALL remain separate operations and
SHALL NOT be fused.

#### Scenario: Modified and deleted tracked files are reported

- **WHEN** the working-tree status query runs against a repository with tracked files that have been
  modified or deleted
- **THEN** the result lists the modified tracked files and the deleted tracked files separately

#### Scenario: Untracked files are excluded

- **WHEN** the working-tree status query runs against a repository containing new untracked files
- **THEN** those untracked files are not included in the result

#### Scenario: A clean tree reports nothing

- **WHEN** the working-tree status query runs against a repository with no uncommitted tracked changes
- **THEN** the result reports no modified and no deleted files

### Requirement: Reconcile marked files against changed files

The system SHALL provide a pure reconciliation that, given the set of files changed since they were
marked, drops those files from the marked set so the reviewer is prompted to revisit them. Files that
have not changed SHALL remain marked. The reconciliation SHALL be a pure function testable without
invoking git.

#### Scenario: A changed marked file is unmarked

- **WHEN** reconciliation runs and a marked file appears in the set of changed files
- **THEN** that file is removed from the marked set

#### Scenario: An unchanged marked file stays marked

- **WHEN** reconciliation runs and a marked file is not in the set of changed files
- **THEN** that file remains marked

#### Scenario: A deleted marked file is unmarked

- **WHEN** reconciliation runs and a marked file has been deleted
- **THEN** that file is removed from the marked set

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
