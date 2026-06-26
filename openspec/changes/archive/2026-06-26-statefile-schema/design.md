## Context

The state file is the JSON serialization of `model.Review` (`backend/model/model.go`). Its structure is defined implicitly by Go struct tags and described informally in `statefile-usage.md`, which is embedded into each file's `_readme`. `storage.go` reads and writes it with `encoding/json` and no validation. The loader already tolerates two legacy on-disk forms via custom `UnmarshalJSON`: bare-path `marked_files` (`["a.go"]`) and pre-anchor comments (`line_number` + `context_before`/`context_line`/`context_after`). Both are upgraded to the current shape in memory on load, so old files open without a migration step.

This change adds a versioned, machine-checkable contract — a CUE schema — and wires validation into save/load, while introducing an optional `version` field so files can be classified against the schema version they were written under. The `cue` CLI is present on this machine but MUST NOT be a runtime dependency: portability (a single self-contained binary) is a stated project goal. See `proposal.md` for motivation and the two spec files for the normative requirements.

## Goals / Non-Goals

**Goals:**
- A SchemaVer-versioned CUE schema, embedded in the binary, describing the canonical `1.0.0` state-file shape.
- A single source of truth for the schema version, read by the Go code rather than duplicated.
- Validation on save (refuse to write non-conforming data) and on load (reject schema-violating files with an actionable, parse-distinct error).
- Classification of a loaded file as unversioned (pre-`1.0.0`), current, or mismatched, so files needing migration are detectable.
- Pure, unit-testable validation and classification functions.

**Non-Goals:**
- Migration of pre-`1.0.0` files (deferred to the future `1.1.0` change, which also makes `version` mandatory).
- Schemas for non-statefile feedback sources (Bitbucket, GitHub, agentic skills, Sonar) — the eventual composition goal, explicitly out of scope here.
- Any change to the meaning or shape of existing fields. The schema documents the current shape; it does not alter it.

## Decisions

### Decision: embed the schema and validate in-process with `cuelang.org/go`
A single `statefile.cue` is embedded with `go:embed` into a new `backend/schema` package, compiled once via `cuecontext`, and used to validate values in-process. Validation encodes the in-memory `*model.Review` directly with `ctx.Encode(review)` (which honours the existing `json` tags), unifies it with the schema definition, and calls `Validate(cue.Concrete(true))`.

- **Why a separate `schema` package**: `model` is deliberately dependency-light (the `_readme` embed was even pushed out to `backend` to keep `model` free of an embed dependency). Importing CUE into `model` would contradict that. `storage.go` (package `main`) imports `code-review/schema`; `model` stays clean.
- **Why `ctx.Encode` over re-marshalling to JSON bytes**: it avoids a second serialization round-trip and validates exactly the value the program holds. Crucially, encoding the *normalized in-memory model* (after `UnmarshalJSON` has upgraded legacy forms) means the schema only has to describe the one canonical `1.0.0` shape — legacy tolerance stays in the loader where it already lives, not duplicated as disjunctions in CUE.
- **Alternative considered — shell out to `cue`**: rejected. It breaks the single-binary portability goal and adds a runtime PATH dependency.
- **Alternative considered — schema accepts both legacy and canonical on-disk forms**: rejected. It would fork the schema into pre-anchor and anchor variants, making the `1.0.0` contract describe history rather than the current model, and would drift from the Go upgrade logic.

### Decision: schema version is defined in CUE and read out by Go
The `.cue` file declares the version once, e.g. `#SchemaVersion: "1.0.0"`, and the versioned definition constrains `version?: #SchemaVersion`. The `schema` package extracts `#SchemaVersion` from the compiled value and exposes it as `schema.Version`. Nothing hard-codes `"1.0.0"` in Go.

- **Why**: satisfies the spec requirement that the version come from the embedded schema, and guarantees the stamped value, the schema constraint, and the classification baseline can never disagree.

### Decision: factor the structural shape from the version layer (CUE narrows, it never overrides)
The schema is structured as a version-agnostic shape composed with a per-version layer, **not** as a single `#Statefile` that bundles the version literal into the shape:

```cue
#SchemaVersion: "1.0.0"

// version-agnostic shape, reused across versions
#ReviewBody: {
    id:           string
    repo_path:    string
    source_branch: string
    target_branch: string
    files:        [...#FileDiff]
    marked_files: [...#FileMark]
    _readme?:     string   // opaque; not otherwise constrained (slated for removal)
    ...                    // open, so a later ADDITION can add optional fields
}

// the 1.0.0 schema: shape + this version's version layer
#Statefile: #ReviewBody & { version?: #SchemaVersion }
```

This is forced by how CUE evaluates, and it is the part most worth recording for future maintainers, because the natural instinct — "extend 1.0.0 and override the bits that changed" — does not work in CUE:

- **CUE unification only narrows; there is no override.** Unifying two values yields their greatest lower bound. `"1.0.0" & "1.1.0"` is ⊥ (bottom/conflict), so you cannot take a definition that pins `version: "1.0.0"` and "extend" it to `1.1.0`. You compose a *new* version from the shared shape instead.
- **Optional → mandatory is legal** (it is a narrowing of presence): `{version?: T} & {version: T}` becomes required. So the planned `1.1.0` move (make `version` mandatory) is expressible by composition.
- **`#`-definitions are closed.** Adding a field absent from a closed definition via unification is an error, so the SchemaVer ADDITION case (new optional fields) also does not work by unifying onto a closed base. Keep the reusable shape **open** (trailing `...`) so additions compose; close only the final per-version definition if strictness is wanted there.

**Recommended approach for future versions** (record this — we will refer back to it):
- A new version is a new definition composed from the shared shape, e.g. `#Statefile_1_1_0: #ReviewBody & { version!: "1.1.0", <new fields> }`. Never attempt to override a prior version's concrete constraints.
- Map the change type to SchemaVer: **ADDITION** (`x.y.Z`) = add optional fields to the shape/version layer; **REVISION** (`x.Y.z`) = tighten a constraint that may invalidate older data (e.g. making `version` mandatory); **MODEL** (`X.y.z`) = a structurally incompatible shape, a distinct definition rather than a composition.
- The Go `schema` package embeds each version's `.cue`, exposes the set of known versions, and the classify step selects the matching definition. The single-schema arrangement here is the degenerate case of that design, so adding versions is additive, not a rewrite.

- **Alternative considered — one `#Statefile` with the version folded in, "extended" per release**: rejected. It collides with both CUE rules above (the version literal conflicts on narrowing; a closed base rejects added fields), so it would force each release to copy the whole shape rather than reuse it.

### Decision: optional `version` on `model.Review`, stamped on save
Add `Version string \`json:"version,omitempty"\`` to `Review`. `SaveReview` stamps it with `schema.Version` on every write, exactly as it already stamps `Readme`. `omitempty` keeps the field optional; a written file always carries it, a pre-`1.0.0` file does not.

- **Why a plain `version`, not `_version`**: a leading-underscore identifier is a CUE *hidden* field, so a `_version` key would have to be quoted in every schema definition to be constrained at all — an easy trap to fall back into as the schema evolves (`_readme` already has to be quoted for this reason, and is being removed). A plain top-level `version` is an ordinary data key with no special handling. The field is new in this change, so there is no on-disk `_version` to migrate from.

### Decision: classify by `version` first, then validate with the matching schema
On load the order is: parse JSON → read `version` → classify → validate.

- **unversioned** (`version` absent): pre-`1.0.0`. Validate against the `1.0.0` schema (its canonical shape, since the loader normalizes legacy fields) and load on success. Record that a future version will require migration.
- **current** (`version` == `schema.Version`): validate against `1.0.0` and load.
- **mismatched** (`version` present and != `schema.Version`): reported as needing migration / written by an incompatible version. It is NOT force-validated against the `1.0.0` schema, which would produce a misleading shape error.

This ordering resolves the apparent tension between the two specs (schema rejects a non-`1.0.0` `version`, yet a mismatch must be *detectable*): the schema constraint guards a current-build file that mislabels itself, while genuine cross-version files are caught by classification before the wrong schema is applied. In this change only the `1.0.0` schema exists, so "mismatched" is surfaced rather than resolved.

## Risks / Trade-offs

- **A pre-anchor legacy file fails `1.0.0` validation before `UnmarshalJSON` runs** → We validate the normalized in-memory model (post-`UnmarshalJSON`), never the raw legacy bytes, so the upgrade happens first and the canonical shape is what is checked.
- **CUE pulls in a non-trivial dependency tree, growing the binary** → Accepted: it is compiled in, so the single-binary portability goal holds; size growth is modest relative to the bundled frontend. No runtime PATH dependency is introduced.
- **Validation on every save/load adds latency** → The schema compiles once at package init and is reused; per-call cost is unifying one small value. Negligible against the speed goal, but the compiled schema MUST be cached, not recompiled per call.
- **Over-strict schema rejects a file the program itself wrote** → Mitigated by validating on save (a write that would not re-load is caught immediately) and by table-driven tests over real fixtures.
- **Adding `version` regenerates the Wails TS model bindings** → A purely additive field; the frontend ignores it. Note the regenerated bindings in the diff.

## Migration Plan

No data migration in this change. Rollout is additive: new files gain `version: "1.0.0"`; existing files keep loading unversioned. Rollback is safe — an older build simply ignores the unknown `version` field on read (it is not in its struct) and omits it on the next write. Migration of pre-`1.0.0` data is the future `1.1.0` change, which makes `version` mandatory.

## Open Questions

- Schema file location: `backend/schema/statefile.cue` (within the package for a local `go:embed`) versus a top-level `schema/` directory shared more widely. Leaning `backend/schema/` for embedding simplicity; revisit when non-statefile schemas arrive.
- Whether the load-time classification result is surfaced to the user/UI now, or merely returned by the loader for a later change to consume. This change only needs it returned/loggable; UI treatment can wait.
