// Schema for the code-review state file, versioned with SchemaVer
// (MODEL-REVISION-ADDITION). See README.md in this directory for the
// evolution rules — in short: CUE narrows and never overrides, so future
// versions are *composed* from #ReviewBody rather than extended.
package statefile

// The current schema version. This is the single source of truth: the Go
// `schema` package reads it out and stamps it onto files, so the stamped
// value, the constraint below, and the classification baseline cannot drift.
#SchemaVersion: "1.0.0"

#CommentStatus: "active" | "resolved" | "ignored"

// An anchor records where a comment's code lived against one git blob.
#Anchor: {
	blob:        string
	line_number: int
	offset?:     int
	context?: [...string]
}

#Comment: {
	id:         string & !=""
	parent_id?: string
	author:     string
	content:    string
	status:     #CommentStatus
	anchors?: [...#Anchor]
}

#FileDiff: {
	file_path: string & !=""
	comments: [...#Comment]
}

#FileMark: {
	path:  string & !=""
	blob?: string
}

// The version-agnostic shape of a review. It is deliberately left open
// (trailing `...`) so a future ADDITION can introduce new optional fields by
// composition. `_readme` is permitted as an opaque string but its content is
// not constrained (the field is slated for removal in a future change).
#ReviewBody: {
	id:            string & !=""
	repo_path:     string
	source_branch: string
	target_branch: string
	// Quoted because a leading-underscore identifier is a CUE *hidden* field;
	// quoting makes "_readme" a regular data key so the constraint applies.
	"_readme"?: string
	files: [...#FileDiff]
	comments?: [...#Comment]
	marked_files: [...#FileMark]
	...
}

// The 1.0.0 state file: the shared shape composed with this version's version
// layer. `version` is optional in 1.0.0; its absence denotes a pre-1.0.0
// file. Do NOT fold the version literal into #ReviewBody — see README.md.
#Statefile: #ReviewBody & {
	// Deliberately named `version`, not `_version`: a leading-underscore
	// identifier is a CUE *hidden* field (see "_readme" above, which must be
	// quoted to escape that), so a plain name keeps this a regular data key the
	// constraint actually applies to.
	version?: #SchemaVersion
}
