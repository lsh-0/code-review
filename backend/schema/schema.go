// Package schema validates code-review state files against an embedded,
// SchemaVer-versioned CUE schema, in-process (no `cue` CLI at runtime). The
// schema itself lives in statefile.cue; the evolution rules are in README.md.
package schema

import (
	_ "embed"
	"fmt"

	"code-review/model"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed statefile.cue
var schemaSource []byte

var (
	cueCtx    *cue.Context
	statefile cue.Value

	// Version is the current schema version, read from #SchemaVersion in the
	// embedded CUE rather than hard-coded here, so the stamped value, the
	// schema constraint, and the classification baseline cannot disagree.
	Version string
)

func init() {
	cueCtx = cuecontext.New()

	root := cueCtx.CompileBytes(schemaSource, cue.Filename("statefile.cue"))
	if err := root.Err(); err != nil {
		panic(fmt.Sprintf("schema: compiling embedded statefile.cue: %v", err))
	}

	statefile = root.LookupPath(cue.ParsePath("#Statefile"))
	if err := statefile.Err(); err != nil {
		panic(fmt.Sprintf("schema: locating #Statefile: %v", err))
	}

	version, err := root.LookupPath(cue.ParsePath("#SchemaVersion")).String()
	if err != nil {
		panic(fmt.Sprintf("schema: reading #SchemaVersion: %v", err))
	}
	Version = version
}

// Validate reports whether the in-memory review conforms to the current
// schema. The review is encoded directly (honouring its `json` tags, including
// the `omitempty` on `version`), so the value checked is the normalized model
// the loader has already produced — the schema only describes the one canonical
// shape, and legacy tolerance stays in the model's UnmarshalJSON. A failure
// names the offending field path.
func Validate(review *model.Review) error {
	data := cueCtx.Encode(review)
	if err := data.Err(); err != nil {
		return fmt.Errorf("encoding review for validation: %w", err)
	}
	if err := statefile.Unify(data).Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("state file does not conform to schema %s: %w", Version, err)
	}
	return nil
}

// Class is how a loaded file's `version` relates to the build's schema version.
type Class int

const (
	// ClassUnversioned: no `version` field — a file written before versioning
	// existed (pre-1.0.0). It still loads in this version.
	ClassUnversioned Class = iota
	// ClassCurrent: `version` equals the build's schema version.
	ClassCurrent
	// ClassMismatched: `version` is present but differs from the build's — the
	// file was written by an incompatible version and needs migration.
	ClassMismatched
)

func (c Class) String() string {
	switch c {
	case ClassUnversioned:
		return "unversioned"
	case ClassCurrent:
		return "current"
	case ClassMismatched:
		return "mismatched"
	default:
		return fmt.Sprintf("Class(%d)", int(c))
	}
}

// Classify places a file's `version` value (empty when absent) against the
// build's schema version. Pure: it depends only on its argument and the
// build-time Version.
func Classify(version string) Class {
	switch version {
	case "":
		return ClassUnversioned
	case Version:
		return ClassCurrent
	default:
		return ClassMismatched
	}
}
