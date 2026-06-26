## ADDED Requirements

### Requirement: Dual-mode invocation

The `code-review` binary SHALL open the GUI when invoked with no subcommand, and
SHALL run the command-line interface when invoked with a recognised subcommand.
The existing `--version` behaviour SHALL be preserved.

#### Scenario: Bare invocation opens the GUI

- **WHEN** the binary is invoked with no arguments
- **THEN** the GUI starts exactly as it does today

#### Scenario: Help reveals subcommands

- **WHEN** the binary is invoked with `-h` or `--help`
- **THEN** it prints the available subcommands and their usage to stdout and exits without opening the GUI

#### Scenario: Subcommand runs the CLI

- **WHEN** the binary is invoked with a recognised subcommand (e.g. `list`)
- **THEN** the CLI handles that command and the GUI does not start

#### Scenario: Unknown subcommand

- **WHEN** the binary is invoked with an unrecognised first argument that is not a flag
- **THEN** it prints an error naming the unknown command and the usage, and exits non-zero

#### Scenario: POSIX-style flags

- **WHEN** a subcommand accepts flags
- **THEN** they are parsed POSIX-style (per-subcommand), and the GUI's existing `--version` behaviour is unaffected

### Requirement: Repository-read-only operation

The CLI SHALL perform read-only git operations only. It MUST NOT switch
branches, stash, commit, revert, or otherwise modify the working tree, index,
or git history. Its only permitted writes are to the review state file.

#### Scenario: No repository mutation

- **WHEN** any CLI command runs
- **THEN** the git working tree, index, current branch, and history are unchanged afterwards

#### Scenario: State file is the only write target

- **WHEN** a mutating command (`resolve`, `reactivate`, `reply`, `unmark`) runs
- **THEN** the only file modified is the review state file under the XDG data directory

### Requirement: Review resolution

The CLI SHALL resolve the review state file from the current working directory's
git root, the current branch, the default branch, and the XDG data directory
(`~/.local/share/code-review`), using the same derivation as the GUI. No flag
SHALL be required or accepted to select a different review.

#### Scenario: Resolves the same review the GUI uses

- **WHEN** the CLI runs inside a repository on a branch for which a review exists
- **THEN** it operates on the same state file the GUI resolves for that repo, branch, and default branch

#### Scenario: Not in a git repository

- **WHEN** the CLI runs outside any git repository
- **THEN** it prints an error explaining a repository is required and exits non-zero

#### Scenario: No review exists for the current branch

- **WHEN** a command other than `start` runs on a branch for which no state file exists
- **THEN** it reports that no review was found for the current branch, directs the reader to `code-review start`, and exits non-zero

### Requirement: Start command

The `start` command SHALL create the review state file for the current
repository and branch when none exists, writing only the state file (read-only
on the repository). It is the only command that creates a review; it SHALL NOT
overwrite an existing review. The review created is empty: it carries no diff or
file list.

#### Scenario: Create a review

- **WHEN** `start` runs on a branch for which no state file exists
- **THEN** it creates the state file at the path the GUI would resolve, reports the branches it started, and exits zero

#### Scenario: Review already exists

- **WHEN** `start` runs on a branch for which a review already exists
- **THEN** it leaves the existing review unchanged, reports that one already exists, and exits zero

#### Scenario: Start outside a repository

- **WHEN** `start` runs outside any git repository
- **THEN** it prints an error explaining a repository is required and exits non-zero

### Requirement: JSON output contract

Read commands SHALL emit purpose-built JSON by default — a stable, documented
shape distinct from the internal state-file schema. The output SHALL NOT require
the agent to know the state file's structure.

#### Scenario: Read output is JSON

- **WHEN** a read command (`list`, `show`, `status`) succeeds
- **THEN** its stdout is valid JSON in the command's documented shape

#### Scenario: Output omits internal schema fields

- **WHEN** a read command emits a comment
- **THEN** the JSON exposes the agent-relevant fields (id, file, line, outdated, author, content, status, replies) and not the raw anchor/blob internals

#### Scenario: Outdated placement is flagged

- **WHEN** a read command emits a comment whose most-recent anchor is adrift
- **THEN** the comment's `outdated` field is true and its `line` is the last reconciled placement

### Requirement: List comments command

The `list` command SHALL output the comments that need the agent's attention —
active root comments — each with the data needed to locate and act on it.

#### Scenario: Lists active comments

- **WHEN** `list` runs and active comments exist
- **THEN** it outputs each active root comment with its id, file path (empty for review-level), current line number, author, and content

#### Scenario: No active comments

- **WHEN** `list` runs and no active comments exist
- **THEN** it outputs an empty JSON array and exits zero

### Requirement: Show comment command

The `show <id>` command SHALL output a single comment together with its full
reply thread and current placement.

#### Scenario: Shows a comment and its thread

- **WHEN** `show <id>` runs for an existing root comment
- **THEN** it outputs that comment's id, file, line, author, content, status, and an ordered list of its replies

#### Scenario: Unknown comment id

- **WHEN** `show <id>` runs for an id that does not exist
- **THEN** it prints an error naming the id and exits non-zero

### Requirement: Status summary command

The `status` command SHALL output a summary of the review: the branches, the
counts of comments by status, and the marked-file count.

#### Scenario: Summarises the review

- **WHEN** `status` runs
- **THEN** it outputs the source and target branches, counts of active/resolved/ignored root comments, and the number of marked files

### Requirement: Resolve and reactivate commands

The `resolve <id>` command SHALL set a root comment's status to resolved, and
`reactivate <id>` SHALL set a resolved comment back to active. Both SHALL persist
the change and report success.

#### Scenario: Resolve an active comment

- **WHEN** `resolve <id>` runs for an existing active root comment
- **THEN** that comment's status becomes resolved, the state file is saved, and the command exits zero

#### Scenario: Reactivate a resolved comment

- **WHEN** `reactivate <id>` runs for an existing resolved root comment
- **THEN** that comment's status becomes active and the state file is saved

#### Scenario: Resolve a reply or unknown id

- **WHEN** `resolve <id>` targets a reply or a non-existent id
- **THEN** it prints an error and exits non-zero without modifying state

### Requirement: Reply command

The `reply <id> <text>` command SHALL append a reply to the identified comment's
thread, authored as the git user, and persist it. It SHALL NOT alter the parent
comment's status.

#### Scenario: Append a reply

- **WHEN** `reply <id> <text>` runs for an existing comment
- **THEN** a new reply with the given text is added to that comment's thread, attributed to the git user, and the state file is saved

#### Scenario: Reply to unknown id

- **WHEN** `reply <id> <text>` targets a non-existent id
- **THEN** it prints an error and exits non-zero without modifying state

### Requirement: Unmark command

The `unmark <file>` command SHALL remove a file from the review's marked-files
set and persist the change. Unmarking a file that is not marked SHALL succeed
without error.

#### Scenario: Unmark a marked file

- **WHEN** `unmark <file>` runs for a file currently in the marked set
- **THEN** that file is removed from the marked set and the state file is saved

#### Scenario: Unmark a file that is not marked

- **WHEN** `unmark <file>` runs for a file not in the marked set
- **THEN** the command exits zero and the state file is unchanged in its marked set

### Requirement: Review-level comment command

The `comment <text>` command SHALL add a review-level (top-level, unattached)
comment authored as the git user, and persist it. The comment SHALL carry no
file path, line number, or context.

#### Scenario: Add a review-level comment

- **WHEN** `comment <text>` runs with non-empty text
- **THEN** a new review-level comment with the given text is added to the review's top-level comments, attributed to the git user, and the state file is saved

#### Scenario: Empty comment text

- **WHEN** `comment` runs with no text
- **THEN** it prints an error and exits non-zero without modifying state

### Requirement: Conservative mutation surface

The CLI SHALL NOT expose actions outside the agent policy: it MUST NOT set a
comment's status to ignored, delete comments or replies, or add a file-anchored
root comment. Only `resolve`, `reactivate`, `reply`, `unmark`, and `comment`
(review-level) mutate the review.

#### Scenario: No ignore or delete commands

- **WHEN** the CLI's command list is shown
- **THEN** it contains no command that ignores a comment, deletes a comment or reply, or adds a file-anchored root comment

### Requirement: Instructions front door

The `instructions` command SHALL print the agent contract — how to use the CLI
and how to conduct the review — sourced from the embedded `backend/instructions.md`.
This text SHALL be the single source for the agent contract. It MUST instruct the
agent to act through the CLI commands and MUST NOT instruct the agent to read or
edit the state file directly.

#### Scenario: Prints the contract

- **WHEN** `instructions` runs
- **THEN** it prints the embedded instructions text to stdout and exits zero

#### Scenario: Contract describes CLI commands, not state edits

- **WHEN** the instructions text is read
- **THEN** every action it directs (resolve, reply, unmark, add review-level comment, find work) is expressed as a `code-review` command, and it contains no description of the state-file schema or of hand-editing the file

### Requirement: State-file readme becomes a pointer

The review state file's `_readme` field SHALL no longer carry the full agent
contract. It SHALL instead carry a short pointer directing the reader to run
`code-review instructions`. The GUI's "copy AI prompt" action SHALL emit the
same pointer.

#### Scenario: Readme points to the CLI

- **WHEN** the state file is written
- **THEN** its `_readme` field instructs the reader to run `code-review instructions` rather than containing the full contract

#### Scenario: Copy-prompt points to the CLI

- **WHEN** the reviewer uses the "copy AI prompt" action
- **THEN** the copied prompt instructs the agent to run `code-review instructions` and follow it
