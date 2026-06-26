# State file schema

`statefile.cue` is the authoritative definition of a valid code-review state
file. It is embedded into the binary (`go:embed`) and evaluated in-process via
the CUE Go library — there is no dependency on the `cue` CLI at runtime.

The schema is versioned with [SchemaVer](https://snowplow.io/blog/introducing-schemaver-for-semantic-versioning-of-schemas/)
(`MODEL-REVISION-ADDITION`). `#SchemaVersion` in the `.cue` file is the single
source of truth for the current version; the Go package reads it out as
`schema.Version` and stamps it onto files on save.

## How to evolve the schema

Read this before changing `statefile.cue`. The instinct carried over from
JSON Schema or object inheritance — "extend the previous version and override
the parts that changed" — does **not** work in CUE, and the schema is
structured the way it is specifically to work with CUE rather than against it.

### CUE narrows; it never overrides

CUE unification computes a greatest lower bound: it can only *add* constraints,
never loosen or replace one. Three consequences govern how we version:

1. **Changing a concrete value is a conflict, not an override.**
   `"1.0.0" & "1.1.0"` is bottom (⊥). You cannot take a definition that pins
   `version: "1.0.0"` and "extend" it to `1.1.0`. This is why the version
   literal is kept out of the reusable shape (see below).
2. **Making an optional field mandatory is a legal narrowing.**
   `{version?: T} & {version: T}` resolves to a required field. So the planned
   `1.1.0` move — making `version` mandatory — is expressible by composition.
3. **`#`-definitions are closed.** Unifying a field that a closed definition
   does not declare is an error. So adding fields (the SchemaVer ADDITION case)
   also does not work by unifying onto a closed base. The reusable shape is
   therefore left **open** (a trailing `...`).

### The structure: shape composed with a version layer

`statefile.cue` separates the version-agnostic shape from the per-version layer:

```cue
#SchemaVersion: "1.0.0"

#ReviewBody: {           // version-agnostic shape, kept open with `...`
    id: string
    // ... the rest of the fields ...
    ...
}

#Statefile: #ReviewBody & { version?: #SchemaVersion }   // the 1.0.0 schema
```

Each version is *composed* from `#ReviewBody`; no version ever overrides another.

### Adding the next version

Define a new top-level definition rather than mutating the existing one, then
have the Go package embed it and select it by `version` in the classify step.
For example, the future `1.1.0` (mandatory `version`, plus any new fields):

```cue
#Statefile_1_1_0: #ReviewBody & {
    version!: "1.1.0"   // mandatory
    // new optional fields, if any
}
```

Map the change to SchemaVer:

- **ADDITION** (`x.y.Z`): add optional fields to `#ReviewBody` or the version
  layer. Backward compatible.
- **REVISION** (`x.Y.z`): tighten a constraint that may invalidate older data
  (e.g. making `version` mandatory). Pair it with migration.
- **MODEL** (`X.y.z`): a structurally incompatible shape — a distinct
  definition, not a composition of `#ReviewBody`.

Migration of older files (and making `version` mandatory) is deferred to that
future change; this version only detects and classifies, it does not migrate.
