## ADDED Requirements

### Requirement: Import command

The `import` command SHALL read an external review-feedback document from a file
path and merge its threads into the review for the current branch, then persist
the result. It SHALL take exactly one positional argument: the path to the
document.

The command SHALL be repository-read-only like every other command: its only write
target is the review state file. It SHALL NOT issue any network request and SHALL
NOT read any credential, so it works offline and cannot reach a hosted provider.

A failure to read, decode, or map the document SHALL leave the review state file
unchanged.

#### Scenario: Imports a document

- **WHEN** `import <file>` runs with a readable, well-formed document
- **THEN** the document's threads are merged into the review
- **AND** the review state file is written

#### Scenario: Reports the outcome

- **WHEN** `import <file>` succeeds
- **THEN** it reports how many comments were imported, placed at a line, left untethered, and skipped
- **AND** it names any file path in the document that is absent from the current diff

#### Scenario: Missing file

- **WHEN** `import` is given a path that does not exist
- **THEN** it errors naming the path and exits non-zero
- **AND** the review state file is unchanged

#### Scenario: Malformed document leaves state untouched

- **WHEN** `import` is given a file that is not a valid document
- **THEN** it errors describing the problem and exits non-zero
- **AND** the review state file is unchanged

#### Scenario: Wrong argument count

- **WHEN** `import` is invoked with no argument or more than one
- **THEN** it prints its usage and exits non-zero

#### Scenario: No network access

- **WHEN** `import` runs
- **THEN** no network request is issued and no credential is read

### Requirement: Imported comments are visible to the agent and act like local ones

The read commands SHALL include imported comments alongside local ones: `list`
SHALL list an active imported root comment, `show` SHALL show an imported comment
with its thread, and `status` SHALL count imported comments in its totals. An
agent working the review therefore sees all of its feedback through one command
set.

The mutating commands SHALL accept an imported comment's id where they accept a
local one: `resolve`, `reactivate` and `reply` SHALL all work on an imported
comment.

#### Scenario: List includes imported comments

- **WHEN** `list` runs and an imported root comment is active
- **THEN** that comment is listed

#### Scenario: Status counts imported comments

- **WHEN** `status` runs on a review with imported comments
- **THEN** its active, resolved and ignored counts include them

#### Scenario: Resolve works on an imported comment

- **WHEN** `resolve <id>` names an imported root comment
- **THEN** the comment is resolved and the state file is written

#### Scenario: Reply works on an imported comment

- **WHEN** `reply <id> <text>` names an imported root comment
- **THEN** a local reply is appended to that comment's thread

## MODIFIED Requirements

### Requirement: JSON output contract

Read commands SHALL emit purpose-built JSON by default — a stable, documented
shape distinct from the internal state-file schema. The output SHALL NOT require
the agent to know the state file's structure.

A comment's origin SHALL be exposed as an `origin` field naming the source it was
imported from, absent or empty for a local comment. The agent needs it to know
before acting that the comment's text cannot be edited; the raw foreign
identifier is an internal matching key and SHALL NOT be exposed.

#### Scenario: Read output is JSON

- **WHEN** a read command (`list`, `show`, `status`) succeeds
- **THEN** its stdout is valid JSON in the command's documented shape

#### Scenario: Output omits internal schema fields

- **WHEN** a read command emits a comment
- **THEN** the JSON exposes the agent-relevant fields (id, file, line, outdated, author, content, status, origin, replies) and not the raw anchor/blob internals or the foreign identifier

#### Scenario: Outdated placement is flagged

- **WHEN** a read command emits a comment whose most-recent anchor is adrift
- **THEN** the comment's `outdated` field is true and its `line` is the last reconciled placement

#### Scenario: Origin identifies an imported comment

- **WHEN** a read command emits an imported comment
- **THEN** its `origin` field names the source it was imported from

#### Scenario: A local comment reports no origin

- **WHEN** a read command emits a locally written comment
- **THEN** its `origin` field is empty or absent

### Requirement: Repository-read-only operation

The CLI SHALL perform read-only git operations only. It MUST NOT switch
branches, stash, commit, revert, or otherwise modify the working tree, index,
or git history. Its only permitted writes are to the review state file.

The CLI SHALL read a file outside the repository only where a command is given
that path as an argument, and SHALL read it without modifying it.

#### Scenario: No repository mutation

- **WHEN** any CLI command runs
- **THEN** the git working tree, index, current branch, and history are unchanged afterwards

#### Scenario: State file is the only write target

- **WHEN** a mutating command (`resolve`, `reactivate`, `reply`, `unmark`, `import`) runs
- **THEN** the only file modified is the review state file under the XDG data directory

#### Scenario: An imported document is not modified

- **WHEN** `import <file>` runs
- **THEN** the document at that path is unchanged afterwards

### Requirement: Conservative mutation surface

The CLI SHALL NOT expose actions outside the agent policy: it MUST NOT set a
comment's status to ignored, delete comments or replies, or add a file-anchored
root comment. Only `resolve`, `reactivate`, `reply`, `unmark`, `comment`
(review-level), and `import` mutate the review.

The CLI SHALL NOT expose a command that edits any comment's content, and SHALL
refuse to change the content of an imported comment through any means, so
imported feedback keeps the text it was written with.

#### Scenario: No ignore or delete commands

- **WHEN** the CLI's command list is shown
- **THEN** it contains no command that ignores a comment, deletes a comment or reply, or adds a file-anchored root comment

#### Scenario: No command edits comment text

- **WHEN** the CLI's command list is shown
- **THEN** it contains no command that changes a comment's content

### Requirement: List comments command

The `list` command SHALL output the comments that need the agent's attention —
active root comments — each with the data needed to locate and act on it. Active
imported root comments SHALL be listed alongside local ones.

#### Scenario: Lists active comments

- **WHEN** `list` runs and active comments exist
- **THEN** it outputs each active root comment with its id, file path (empty for review-level), current line number, author, content, and origin

#### Scenario: No active comments

- **WHEN** `list` runs and no active comments exist
- **THEN** it outputs an empty JSON array and exits zero

#### Scenario: Imported and local comments are listed together

- **WHEN** `list` runs on a review holding both local and imported active comments
- **THEN** both appear in the output, each carrying its own origin
