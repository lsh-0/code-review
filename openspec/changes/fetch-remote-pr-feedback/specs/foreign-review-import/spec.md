## MODIFIED Requirements

### Requirement: The import format mirrors the upstream normalised encoding

The system SHALL accept an import document in the JSON encoding produced by the
upstream `bb-feedback` program: an object with a `pull_request` object, a
`discussion` array of threads, and a `file_feedback` array of threads. Each
thread SHALL carry a `location`, a `resolution`, and a `root` comment whose
`replies` array holds its nested replies. The published import document schema
SHALL be the normative definition of this encoding.

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

## ADDED Requirements

### Requirement: The import document has a published schema

The system SHALL publish a JSON Schema (draft 2020-12) for the import document,
with a stable `$id`. It SHALL define every property, its type, which properties
are required, which may be `null`, the closed set of values for each tag, and
which properties each tag requires. The schema SHALL be committed in the
repository and embedded in the binary.

#### Scenario: Upstream output conforms

- **WHEN** a document produced by upstream `bb-feedback` is validated against the schema
- **THEN** it is valid

#### Scenario: Each tag's properties are required

- **WHEN** a position of kind `unchanged` lacks `updated_lines`
- **THEN** the document is invalid against the schema

### Requirement: Documents are validated against the schema before import

The system SHALL validate every import document against the embedded schema
before decoding it, whatever produced it. A document that fails validation SHALL
be rejected and the review state file SHALL NOT be written. Each violation SHALL
be reported with its JSON Pointer location in the document and the constraint it
broke.

#### Scenario: A violation names its location

- **WHEN** a document's `file_feedback/0/root/author/kind` is `robot`
- **THEN** the import fails with an error containing `/file_feedback/0/root/author/kind` and the allowed values

#### Scenario: Several violations are all reported

- **WHEN** a document has two independent violations
- **THEN** the error names both locations

### Requirement: Unknown properties are rejected

A document carrying a property the schema does not define SHALL be invalid. A
misspelt property name is therefore reported instead of being ignored.

#### Scenario: A misspelt top-level property

- **WHEN** a document carries `file_feedbacks` in place of `file_feedback`
- **THEN** the import fails with an error naming `file_feedbacks` as not allowed

### Requirement: Documents carry an optional schema version

A document MAY carry a top-level integer `schema_version`. An absent
`schema_version` SHALL be read as version 1. A version other than 1 SHALL be
rejected with an error naming the version found and the versions supported.

#### Scenario: An unversioned document is version 1

- **WHEN** a document without `schema_version` is imported
- **THEN** it is validated as version 1

#### Scenario: An unsupported version

- **WHEN** a document carries `"schema_version": 2`
- **THEN** the import fails with an error naming version 2 and the supported version 1

### Requirement: Rules the schema cannot express are documented and enforced

A span's `first` SHALL NOT exceed its `last`. JSON Schema cannot compare two
properties, so the decoder SHALL enforce this rule after schema validation. The
schema SHALL state the rule in the span's `description`. No other rule SHALL be
enforced outside the schema.

#### Scenario: An inverted span passes the schema but is rejected

- **WHEN** a span has `first` 20 and `last` 12
- **THEN** the import fails with an error naming the span's location and both values

#### Scenario: The schema states the rule

- **WHEN** the schema's span definition is read
- **THEN** its `description` states that `first` does not exceed `last`
