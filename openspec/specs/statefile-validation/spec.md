# statefile-validation Specification

## Purpose
Integrate the versioned CUE schema into the state file's load and save path so that what is written conforms and what is read is checked. Validation is evaluated in-process via the CUE Go library, with no dependency on an external `cue` CLI at runtime. On save, a non-conforming review is refused; on load, a non-conforming file fails with an actionable error that distinguishes a schema-shape failure from a parse failure. On load the file's `version` is also read and classified against the build's schema version (unversioned, current, or mismatched) so files needing migration can be flagged — detection only, no migration is performed.

## Requirements

### Requirement: Validate against the embedded schema in-process
The program SHALL validate state files against the embedded CUE schema using the CUE Go library (`cuelang.org/go`), evaluated in-process. The program SHALL NOT depend on an external `cue` CLI at runtime.

#### Scenario: Validation runs without external tooling
- **WHEN** the program validates a state file on a machine with no `cue` binary on the PATH
- **THEN** validation still runs and produces a result

### Requirement: Validate on save
Before a state file is written, the program SHALL validate the serialized review against the schema. A review that does not conform SHALL NOT be written, and the save SHALL fail with an error identifying the offending field path.

#### Scenario: Conforming review is written
- **WHEN** a review that conforms to the schema is saved
- **THEN** the file is written and the save succeeds

#### Scenario: Non-conforming review is not written
- **WHEN** a save is attempted for a review that does not conform to the schema
- **THEN** no file is written and the save returns an error naming the failing path

### Requirement: Validate on load
When the program loads a state file, it SHALL validate the parsed content against the schema. A file that does not conform SHALL fail to load with an actionable error that distinguishes a schema-shape failure from a parse failure. A JSON type mismatch is a parse failure, rejected during decoding; a value that decodes but violates a constraint (a bad `status`, an empty `id`, a non-matching `version`) is a schema failure whose error names the offending field path.

#### Scenario: Conforming file loads
- **WHEN** a state file that conforms to the schema is loaded
- **THEN** the review is returned with no validation error

#### Scenario: Malformed file is rejected on load
- **WHEN** a state file that parses as JSON but violates the schema is loaded
- **THEN** loading fails with an error that identifies the offending path and is distinct from a JSON parse error

### Requirement: Detect version and flag files needing migration
On load, the program SHALL read the file's `version` and compare it to the current schema version (`1.0.0`). A file MUST be classified as one of: unversioned (no `version`, i.e. pre-`1.0.0`), current (`version` equals the build's schema version), or mismatched (`version` present but not equal). An unversioned file SHALL still load in this change; classification records that migration will be required by a future version. Detection only — this change performs no migration.

#### Scenario: Unversioned file is classified pre-1.0.0 and still loads
- **WHEN** a conforming state file without a `version` field is loaded
- **THEN** it loads successfully and is classified as unversioned (pre-`1.0.0`)

#### Scenario: Current-version file is classified current
- **WHEN** a state file with `version: "1.0.0"` is loaded against the `1.0.0` build
- **THEN** it loads successfully and is classified as current

#### Scenario: Mismatched version is detectable
- **WHEN** a state file carries a `version` other than the build's schema version
- **THEN** the program can report the mismatch as a file that needs migration rather than silently accepting it
