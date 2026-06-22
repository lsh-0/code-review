## MODIFIED Requirements

### Requirement: Separate review-state reload from diff recompute

The system SHALL expose two distinct backend operations: a review-state reload that re-reads the
state JSON without invoking git, and a diff recompute that runs `git diff` and re-parses the result.
Neither operation SHALL do the other's work.

The diff recompute SHALL, after re-parsing the diff, reconcile every comment's anchors against its
file's current blob SHA, so a comment is re-placed, recovered, or marked outdated as the underlying
content changes. The review-state reload SHALL NOT perform anchor reconciliation, since it does no git
work and has no current blob SHAs to reconcile against.

#### Scenario: Reloading review state does not invoke git

- **WHEN** the review-state reload operation runs
- **THEN** the state JSON is re-read into memory
- **AND** no git subprocess is invoked
- **AND** no anchor reconciliation occurs

#### Scenario: Recomputing the diff runs git

- **WHEN** the diff recompute operation runs
- **THEN** `git diff` between the review's branches is executed and parsed
- **AND** the file-content cache is reset

#### Scenario: Recomputing the diff reconciles comment anchors

- **WHEN** the diff recompute operation runs and a commented file's content has changed since a comment's most-recent anchor
- **THEN** that comment is reconciled against the file's current blob SHA
- **AND** the comment is re-placed, recovered, or marked outdated according to the reconciliation outcome
