## Why

The state file is the program's only persisted artifact, yet its shape lives implicitly in the `model.Review` Go structs and a prose `_readme`. There is no machine-checkable contract for what a valid state file looks like, and no version stamped into a file, so an older file that *used* to be valid but no longer is cannot be distinguished from a corrupt one. As we move toward composing review feedback from several origins (Bitbucket, GitHub, agentic review skills, SonarQube/SonarCloud), we need a single, documented, versioned definition of "code review data" to build on. This change establishes that foundation for the current state file only.

## What Changes

- Introduce **CUE** as a project dependency, embedded via its Go library (`cuelang.org/go`) rather than shelling out to the `cue` CLI, and add a versioned CUE schema that describes a valid state file (the current `Review` shape: review metadata, files, comments, anchors, marked files).
- Version the schema with **SchemaVer** (`MODEL-REVISION-ADDITION`), starting at **1.0.0**, so a state file can be matched against the schema version it was written under and flagged when it predates the current model.
- Add an **optional `version`** field stamped into the state file on write (set to `1.0.0`) and read back on load. Its absence is meaningful: an unversioned file is, by definition, pre-`1.0.0`. A future `1.1.0` (out of scope) will make `version` mandatory — at which point unversioned, pre-`1.0.0` data may be blocked from interaction until migrated — and will introduce migration tooling. This change only lays the groundwork: detection, not migration.
- Integrate validation into the load/save path so a written state file conforms to the schema, and a loaded file is checked against it with a clear, actionable error when it does not.
- Capture the schema as the authoritative documentation of the state-file structure.

This is **non-breaking** for existing state files: `version` is optional in `1.0.0`, so files without it are treated as the pre-`1.0.0` baseline and continue to load.

## Capabilities

### New Capabilities
- `statefile-schema`: the versioned CUE definition of a valid state file, the SchemaVer convention governing it, and the version identifier stamped into each file.
- `statefile-validation`: validating a state file against the schema on save and load, and detecting a version mismatch that marks a file as needing migration.

### Modified Capabilities
<!-- None: no existing spec's requirements change. The state-file shape is being documented and validated, not altered. -->

## Impact

- **New dependency**: `cuelang.org/go` added to `backend/go.mod`, embedded as a library (the embedded CUE schema is evaluated in-process; no `cue` CLI at runtime).
- **New schema artifact(s)**: a versioned CUE schema file, likely under a dedicated schema directory, treated as the source of truth for state-file structure.
- **`backend/model`**: an optional `version` field added to `Review` (stamped to `1.0.0` on write, read on load), consistent with the existing non-migrating load behaviour for legacy fields.
- **`backend/storage.go`**: `SaveReview`/`LoadReview` gain validation and version handling.
- **Tests**: schema validation and version-detection covered by unit tests over fixture state files (valid, legacy/unversioned, and invalid).
