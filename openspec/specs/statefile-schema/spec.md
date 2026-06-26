# statefile-schema Specification

## Purpose
Give the state file — the program's only persisted artifact — a machine-checkable, versioned contract instead of a shape that lives implicitly in the `model.Review` Go structs and a prose `_readme`. The capability defines the current `Review` shape as a CUE schema embedded in the binary, versions that schema with SchemaVer (`MODEL-REVISION-ADDITION`) starting at `1.0.0`, and stamps an optional `version` identifier into each file so a file written under an older model can be distinguished from a corrupt one. This is the foundation for composing review feedback from several origins; it establishes the definition and version stamp only, with no migration.

## Requirements

### Requirement: Versioned CUE schema for the state file
The project SHALL define the structure of a valid state file as a CUE schema, embedded in the binary, that describes the current `Review` shape: review metadata (`id`, `repo_path`, `source_branch`, `target_branch`), the `files` array of file diffs, file and review-level `comments` with their `anchors`, and the `marked_files` set. The schema SHALL permit an optional `_readme` field as an opaque string but SHALL NOT constrain its content (the field is slated for removal in a future change). The CUE schema SHALL be the authoritative definition of state-file structure.

Validation runs against the decoded review, so a JSON type mismatch is caught during decoding (a parse failure) before the schema is applied; the schema enforces value-level and structural constraints — enumerations, non-empty identifiers, the `version` literal, and overall shape.

#### Scenario: Schema accepts a well-formed state file
- **WHEN** a state file produced by the current program is evaluated against the schema
- **THEN** evaluation succeeds with no errors

#### Scenario: Schema rejects a structurally invalid state file
- **WHEN** a state file with a disallowed value (for example a comment `status` outside `active`/`resolved`/`ignored`) or an empty required identifier (a comment with no `id`) is evaluated against the schema
- **THEN** evaluation fails and reports the offending path

### Requirement: SchemaVer versioning of the schema
The schema SHALL be versioned using SchemaVer (`MODEL-REVISION-ADDITION`) and SHALL begin at version `1.0.0`. The current schema version SHALL be a single source of truth available to the program at build time.

#### Scenario: Current schema version is exposed
- **WHEN** the program needs the schema version it was built with
- **THEN** it obtains `1.0.0` from the embedded schema definition rather than a hard-coded literal duplicated elsewhere

### Requirement: Optional `version` field
The schema SHALL define an optional `version` field on the state file. When present, `version` MUST equal the schema version the file was written under (`1.0.0` in this change). The absence of `version` SHALL be valid and SHALL denote a file written before versioning existed (pre-`1.0.0`).

#### Scenario: File stamped with the current version is valid
- **WHEN** a state file carries `version: "1.0.0"` and otherwise conforms
- **THEN** it validates against the schema

#### Scenario: File without `version` is valid
- **WHEN** a state file omits `version` and otherwise conforms
- **THEN** it validates against the schema and is treated as pre-`1.0.0`

#### Scenario: File with a non-matching `version` is rejected
- **WHEN** a state file carries a `version` that is not `1.0.0`
- **THEN** evaluation against the `1.0.0` schema fails

### Requirement: Version stamped on write
When the program writes a state file, it SHALL stamp `version` with the current schema version (`1.0.0`). A state file written and then re-read by the program SHALL carry `version`.

#### Scenario: Saved file carries the current version
- **WHEN** the program saves a review
- **THEN** the written file contains `version: "1.0.0"`
