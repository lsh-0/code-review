## ADDED Requirements

### Requirement: The import format mirrors the upstream normalised encoding

The system SHALL accept an import document in the JSON encoding produced by the
upstream `bb-feedback` program: an object with a `pull_request` object, a
`discussion` array of threads, and a `file_feedback` array of threads. Each
thread SHALL carry a `location`, a `resolution`, and a `root` comment whose
`replies` array holds its nested replies.

The format is provider-independent: it SHALL NOT be described, named, or decoded
as Bitbucket-specific data, so a second source normalised onto the same encoding
imports through the same code path.

The system SHALL decode the format's tagged unions by their tag names and SHALL
reject a document carrying an unrecognised tag with an error naming the tag and
the field it appeared in. An unrecognised tag SHALL NOT be defaulted to a known
case, because the distinctions the tags draw (a human author against an
automated one, an old-side position against a new-side one) are exactly what
would be misreported.

#### Scenario: A well-formed document is accepted

- **WHEN** an import document with `pull_request`, `discussion` and `file_feedback` is read
- **THEN** it decodes without error
- **AND** the threads in `discussion` are treated as review-level feedback
- **AND** the threads in `file_feedback` are treated as file-anchored feedback

#### Scenario: An unrecognised tag is rejected

- **WHEN** an import document contains a `position.kind`, `body.kind`, `author.kind`, `location.kind` or `resolution.state` outside the known set
- **THEN** the import fails with an error naming the unrecognised tag and its field
- **AND** the review state file is not written

#### Scenario: A malformed document is rejected without a partial write

- **WHEN** an import document is not valid JSON, or is missing a required field
- **THEN** the import fails with an error
- **AND** the review state file is not written

#### Scenario: A thread's declared collection does not override its location

- **WHEN** a thread with a `file_revision` location appears in the `discussion` array
- **THEN** it is treated as file-anchored feedback according to its `location`, not according to the array it appeared in

### Requirement: Imported comments carry their origin

An imported comment SHALL carry an origin recording the source it was imported
from and its identifier within that source. A comment with no origin SHALL be a
local comment: one written in this tool.

The origin SHALL be omitted from the serialised state file when empty, so an
existing state file remains valid and every comment already in it remains local
without a migration step.

#### Scenario: An imported comment records its source and foreign identifier

- **WHEN** a thread is imported from a source
- **THEN** every comment created from that thread carries an origin naming the source and that comment's identifier in the source document

#### Scenario: A local comment has no origin

- **WHEN** a reviewer adds a comment in the GUI, or an agent adds one through the CLI
- **THEN** the comment carries no origin
- **AND** the serialised comment has no origin field

#### Scenario: An existing state file loads with every comment local

- **WHEN** a state file written before this change is loaded
- **THEN** it loads without error
- **AND** every comment in it is a local comment

### Requirement: Imported comment text is immutable

The system SHALL refuse to edit the content of an imported comment, and SHALL
refuse to delete an imported comment, through every surface: the GUI bridge and
the CLI alike. The refusal SHALL name the reason and SHALL leave the review state
unchanged.

The system SHALL permit every other action on an imported comment: adding a local
reply to it, and resolving, ignoring or reactivating it.

#### Scenario: Editing an imported comment is refused

- **WHEN** an edit is requested for a comment carrying an origin
- **THEN** the request fails with an error stating the comment was imported and its text cannot be changed
- **AND** the review state file is not written

#### Scenario: Deleting an imported comment is refused

- **WHEN** a delete is requested for a comment carrying an origin
- **THEN** the request fails with an error stating the comment was imported
- **AND** the review state file is not written

#### Scenario: Resolving an imported comment succeeds

- **WHEN** an imported root comment is resolved
- **THEN** its status becomes resolved
- **AND** the change is persisted

#### Scenario: Replying to an imported comment succeeds

- **WHEN** a reply is added to an imported root comment
- **THEN** the reply is appended to that comment's thread
- **AND** the reply is a local comment carrying no origin
- **AND** the reply can itself be edited and deleted

### Requirement: Imported reply threads are preserved by re-rooting

The import format nests replies to arbitrary depth; the review model's threads are
flat, one level of replies under a root. The system SHALL preserve every imported
reply by re-rooting it: a reply at any depth SHALL become a reply whose parent is
the thread's root comment.

Re-rooted replies SHALL retain their order, so a nested exchange reads in the
order it was written. Each SHALL carry its own origin and its own author, so the
authorship of a multi-party thread survives the flattening.

This is the same rule the system already applies when a reviewer replies to a
reply locally, so imported and local threads have one flattening rule rather than
two.

#### Scenario: A deeply nested reply becomes a reply to the root

- **WHEN** a thread is imported whose root has a reply that itself has a reply, three or more levels deep
- **THEN** every reply in that thread is a reply to the root comment
- **AND** no imported comment is a reply to a reply

#### Scenario: Reply order is preserved

- **WHEN** a nested thread is imported
- **THEN** its replies appear in the order they were written in the source document

#### Scenario: Reply authorship is preserved

- **WHEN** a thread with replies from several authors is imported
- **THEN** each reply retains its own author

### Requirement: Imported comments are placed at their stated line

A file-anchored imported comment with a new-side position SHALL be placed at the
first line of that position's span, and SHALL be given a first anchor built from
the current diff at that line: the file's current blob SHA, the stated line
number, a captured context window, and the anchored line's offset within it. That
makes the imported comment behave exactly as a comment created locally at that
line now, and lets the existing reconciler carry it forward as the file changes.

A multi-line span SHALL be placed at its first line. The remaining lines of the
span SHALL NOT create additional comments.

When the stated line is absent from the current diff, the system SHALL give the
comment an adrift first anchor carrying only the current blob SHA, so the comment
renders untethered rather than being placed on unrelated code or discarded.

#### Scenario: A single-line new-side comment is placed at its line

- **WHEN** a thread with an `updated` position covering one line is imported and that line is present in the diff
- **THEN** the comment's first anchor records the current blob SHA, that line number, a captured context window, and the anchored line's offset
- **AND** the comment is not outdated

#### Scenario: A multi-line span is placed at its first line

- **WHEN** a thread with a position spanning several lines is imported
- **THEN** one comment is created, anchored at the span's first line

#### Scenario: An unchanged-context position uses its new-side line

- **WHEN** a thread with an `unchanged` position, carrying both an original and an updated span, is imported
- **THEN** the comment is placed at the first line of the updated span

#### Scenario: A line absent from the diff yields an untethered comment

- **WHEN** a thread's stated line is not present in the current diff
- **THEN** the comment's first anchor is adrift, carrying only the current blob SHA
- **AND** the comment is outdated and renders untethered

### Requirement: Positions the model cannot express are reported

An old-side (`original`) position addresses lines the change removed, and a
whole-file position addresses no line. The review model anchors only to new-side
lines, so neither has a placement.

The system SHALL import such a thread as an untethered comment on its file, with
an adrift first anchor, rather than discarding it. The import report SHALL count
these separately and name each affected file, so the reviewer knows the comment
is present but could not be placed.

#### Scenario: An old-side position imports untethered

- **WHEN** a thread with an `original` position is imported
- **THEN** the comment is attached to its file with an adrift first anchor
- **AND** the import report counts it as unplaceable and names its file

#### Scenario: A whole-file position imports untethered

- **WHEN** a thread with a `whole_file` position is imported
- **THEN** the comment is attached to its file with an adrift first anchor
- **AND** the import report counts it as unplaceable and names its file

### Requirement: Deleted and pending source comments are not imported

A source comment whose body is marked deleted carries no text. A source comment
marked pending is an unpublished draft that the source's own reviewers cannot see.

The system SHALL skip both, and SHALL count each skipped comment in the import
report. Skipping a root SHALL skip its replies with it; skipping a reply SHALL
leave the rest of its thread intact.

#### Scenario: A deleted comment is skipped and counted

- **WHEN** a thread contains a comment whose body is deleted
- **THEN** that comment is not imported
- **AND** the import report counts it as skipped

#### Scenario: A pending comment is skipped and counted

- **WHEN** a thread contains a comment marked pending
- **THEN** that comment is not imported
- **AND** the import report counts it as skipped

#### Scenario: Skipping a root skips its thread

- **WHEN** a thread's root comment is deleted or pending
- **THEN** neither the root nor any of its replies is imported

### Requirement: The review records its imported sources

The review SHALL hold a list of the sources imported into it. Each source SHALL
record its identifier, a display label for the UI, the pull request it came from,
and when it was imported.

The source list SHALL be absent from the serialised state file when empty, so a
review with no imports is unchanged on disk.

The source's recorded pull request SHALL include the source and destination branch
names, so a reviewer can see whether an import belongs to the branches under
review.

#### Scenario: An import registers its source

- **WHEN** a document is imported
- **THEN** the review's source list gains an entry recording the source identifier, a display label, the pull request identifier and title, its source and destination branches, and the import time

#### Scenario: A review with no imports has no source list on disk

- **WHEN** a review with no imported sources is written
- **THEN** the serialised state file has no source list field

### Requirement: Re-importing a source replaces its feedback

Importing a document for a source already present in the review SHALL replace that
source's imported comments rather than duplicating them. The system SHALL match
existing imported comments to the incoming ones by their foreign identifiers.

Local replies to a re-imported comment SHALL be preserved and SHALL remain
attached to it. Local status changes on a re-imported comment SHALL be preserved,
so a thread resolved locally does not revert to active on re-import.

An imported comment absent from the incoming document SHALL be removed, together
with its local replies, since the thread no longer exists at the source.

#### Scenario: Re-importing does not duplicate feedback

- **WHEN** the same document is imported twice
- **THEN** the review holds one copy of each imported comment
- **AND** the source's registry entry records the later import time

#### Scenario: Local replies survive a re-import

- **WHEN** a local reply has been added to an imported comment, and that source is re-imported with the comment still present
- **THEN** the reply is still attached to that comment

#### Scenario: Local resolution survives a re-import

- **WHEN** an imported comment has been resolved locally, and that source is re-imported with the comment still present
- **THEN** the comment is still resolved

#### Scenario: A comment removed at the source is removed locally

- **WHEN** a source is re-imported and a previously imported comment is absent from the incoming document
- **THEN** that comment is removed from the review
- **AND** its local replies are removed with it

#### Scenario: Importing a second source leaves the first intact

- **WHEN** a document is imported for a source not already present
- **THEN** the previously imported sources and their comments are unchanged

### Requirement: The import reports what it mapped and what it could not

The system SHALL report the outcome of an import: the number of comments
imported, the number placed at a line, the number untethered, the number skipped,
and the number of files whose paths are not present in the current diff.

A file path in the document that is absent from the current diff SHALL be
reported and SHALL name the path, because comments on such a file do not appear
in the file list or the overview. A document in which no path matches the current
diff SHALL be reported as such, since it indicates the document belongs to a
different branch or repository.

#### Scenario: A successful import reports its counts

- **WHEN** a document is imported
- **THEN** the report states how many comments were imported, placed, untethered and skipped

#### Scenario: Unmatched file paths are named

- **WHEN** a document contains feedback on a file absent from the current diff
- **THEN** the report names that path and counts it as unmatched

#### Scenario: A wholly unmatched document is called out

- **WHEN** no file path in the document is present in the current diff
- **THEN** the report states that no path matched the current diff
