## MODIFIED Requirements

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

- **WHEN** a mutating command (`resolve`, `reactivate`, `reply`, `unmark`, `import`, `fetch`) runs
- **THEN** the only file modified is the review state file under the XDG data directory

#### Scenario: An imported document is not modified

- **WHEN** `import <file>` runs
- **THEN** the document at that path is unchanged afterwards

### Requirement: Conservative mutation surface

The CLI SHALL NOT expose actions outside the agent policy: it MUST NOT set a
comment's status to ignored, delete comments or replies, or add a file-anchored
root comment. Only `resolve`, `reactivate`, `reply`, `unmark`, `comment`
(review-level), `import` and `fetch` mutate the review.

The CLI SHALL NOT expose a command that edits any comment's content, and SHALL
refuse to change the content of an imported comment through any means, so
imported feedback keeps the text it was written with.

#### Scenario: No ignore or delete commands

- **WHEN** the CLI's command list is shown
- **THEN** it contains no command that ignores a comment, deletes a comment or reply, or adds a file-anchored root comment

#### Scenario: No command edits comment text

- **WHEN** the CLI's command list is shown
- **THEN** it contains no command that changes a comment's content

## ADDED Requirements

### Requirement: Fetch command

The `fetch` command SHALL retrieve review feedback from the hosted pull request
for the current branch and merge it into the review, then persist the result. It
SHALL take zero or one positional argument. With none, it discovers the pull
request. With one, it treats the argument as a pull request URL. It SHALL print
the import report to standard output, and a failure SHALL leave the state file
unchanged.

#### Scenario: Fetches the current branch's pull request

- **WHEN** `fetch` runs with no argument and one open pull request matches the current branch
- **THEN** its feedback is merged into the review, the state file is written, and the report is printed

#### Scenario: Fetches a named pull request

- **WHEN** `fetch <url>` runs with a valid pull request URL
- **THEN** that pull request's feedback is merged and the report is printed

#### Scenario: Failure leaves state untouched

- **WHEN** `fetch` fails to locate, retrieve or decode the pull request
- **THEN** it prints the error to standard error and exits non-zero
- **AND** the review state file is unchanged

#### Scenario: Wrong argument count

- **WHEN** `fetch` is invoked with more than one argument
- **THEN** it prints its usage and exits non-zero

### Requirement: Schema command

The `schema` command SHALL print the embedded import document schema to standard
output and exit zero. It SHALL take no arguments, work outside a repository, and
issue no network request.

#### Scenario: Prints the schema

- **WHEN** `schema` runs
- **THEN** standard output is the embedded JSON Schema, byte-identical to the committed file

#### Scenario: Works outside a repository

- **WHEN** `schema` runs in a directory that is not a git repository
- **THEN** it prints the schema and exits zero

### Requirement: Fetch is the only command with network access

`fetch` SHALL be the only CLI command that issues a network request or reads a
credential. Its requests SHALL go only to the provider's API host, over HTTPS,
with certificates verified against the system trust store. Every other command,
`import` included, SHALL work offline.

#### Scenario: Other commands stay offline

- **WHEN** any command other than `fetch` runs
- **THEN** no network request is issued and no credential is read

#### Scenario: Certificate verification is not disabled

- **WHEN** `fetch` connects to the provider
- **THEN** a certificate that fails verification aborts the request with a transport error
