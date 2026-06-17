## ADDED Requirements

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
