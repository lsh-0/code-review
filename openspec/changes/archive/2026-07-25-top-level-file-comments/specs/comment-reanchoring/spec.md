## MODIFIED Requirements

### Requirement: Outdated comments render untethered with a warning indicator

The system SHALL render an outdated comment detached from any live hunk, positioned at the top of its file's view, displaying its most-recent good context as a read-only pseudo-hunk marked with a warning (yellow) border. An untethered outdated comment SHALL NOT offer a line-expand affordance. When a file also has top-level (anchorless) comments, the untethered outdated comments SHALL render **below** the file's top-level comments and **above** the file's code, giving a top-of-file order of top-level comments, then untethered comments, then code.

#### Scenario: Outdated comment shows its captured context at the top of the file

- **WHEN** a file with an outdated comment is rendered
- **THEN** the outdated comment appears at the top of the file's view as a read-only pseudo-hunk built from its most-recent good context
- **AND** the pseudo-hunk has a warning border and no line-expand control

#### Scenario: A non-outdated comment renders against its line as before

- **WHEN** a file's comment has a good most-recent anchor
- **THEN** the comment renders against its anchored line in the live diff, not at the top of the file

#### Scenario: Untethered comments sit below top-level comments

- **WHEN** a file has both a top-level (anchorless) comment and an outdated comment
- **THEN** the outdated comment's untethered block renders below the top-level comment
- **AND** above the file's code
