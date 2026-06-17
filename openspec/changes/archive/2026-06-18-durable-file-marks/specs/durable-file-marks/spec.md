## ADDED Requirements

### Requirement: A mark records the reviewed version of the file

The system SHALL, when the reviewer marks a file as done, record alongside the path the git blob SHA of
that file's content at the review's source-branch HEAD. The signature SHALL be the committed content, so
that uncommitted working-tree edits do not change it.

#### Scenario: Marking stores the current blob signature

- **WHEN** the reviewer marks a file as done
- **THEN** the marked set records the file's path together with its blob SHA at the source-branch HEAD

#### Scenario: Unmarking removes the record

- **WHEN** the reviewer unmarks a file
- **THEN** the file's record is removed from the marked set

### Requirement: A changed file is unmarked when the review is opened

The system SHALL, on opening a review, compare each mark's stored blob SHA against the file's current
blob SHA at the source-branch HEAD, and evict any mark whose file has changed or been deleted. This
SHALL hold across application restarts, so a file changed while the application was closed is no longer
shown as marked when it reopens.

#### Scenario: A file changed between sessions is unmarked

- **WHEN** a file was marked, the application was closed, the file was changed by a commit, and the
  application is reopened
- **THEN** that file is no longer marked

#### Scenario: A deleted marked file is unmarked

- **WHEN** a marked file no longer exists at the source-branch HEAD on opening the review
- **THEN** that file's mark is evicted

#### Scenario: An unchanged marked file stays marked

- **WHEN** a marked file's current blob SHA matches its stored SHA on opening the review
- **THEN** that file remains marked

#### Scenario: Uncommitted edits do not unmark

- **WHEN** a marked file has uncommitted working-tree edits but its committed content at the
  source-branch HEAD is unchanged
- **THEN** that file remains marked

### Requirement: Legacy bare-path marks are migrated on load

The system SHALL load existing state files whose `marked_files` is a bare list of paths, and backfill
each such mark with the file's current blob SHA at the source-branch HEAD on first open, establishing a
baseline. A backfilled mark SHALL NOT be evicted on that first open merely for lacking a prior
signature.

#### Scenario: A legacy mark is backfilled, not dropped

- **WHEN** a state file with bare-path marks is opened
- **THEN** each existing mark gains the file's current blob SHA
- **AND** the file remains marked on that first open

#### Scenario: A backfilled mark is durable thereafter

- **WHEN** a backfilled mark's file is later changed by a commit and the review is reopened
- **THEN** that file is unmarked
