## Why

Review feedback raised on a hosted pull request (Bitbucket today, GitHub later) is
awkward to work through on the platform, and the local tool cannot see it. A
companion program (`bb-feedback`, in `experiments/ocaml-lang`) already fetches a
Bitbucket pull request and normalises it into a provider-independent JSON
document, so the remaining gap is on this side: read that document, map it onto
the review model, and render it alongside local feedback.

Importing it here puts hosted feedback into the same review→fix→resolve loop the
local comments already have, with the CLI available to an agent. This is the
long-standing TODO item "Import review feedback from Bitbucket and GitHub",
scoped down to a one-way file import with no network, no credentials, and no
provider API clients.

## What Changes

- A new import format mirroring the upstream `Code_feedback` JSON encoding
  (`pull_request`, `discussion`, `file_feedback`). The format is
  provider-independent upstream, so a future GitHub source normalises onto it and
  this importer is unchanged.
- Imported threads become ordinary `model.Comment` values in the existing
  per-file and review-level comment lists, each carrying a new `Origin` record
  naming the source it came from and its identifier there. A comment with an
  empty `Origin` is local, so every existing state file stays valid and every
  existing comment keeps its meaning.
- Imported comment text is read-only: it cannot be edited or deleted, in the GUI
  or the CLI. Everything else works normally — a local reply can be added to an
  imported thread, and an imported thread can be resolved, ignored and
  reactivated locally.
- Imported reply threads are preserved. The upstream shape nests replies
  arbitrarily deep; they map onto the model's flat thread by re-rooting, which is
  the same rule `appendReply` already applies to a local reply-to-a-reply.
- A source registry on the review records each import: its id, a display label,
  the pull request it came from, and when it was imported. Re-importing the same
  source updates it in place rather than duplicating the feedback, keyed on the
  foreign comment ids.
- A `code-review import <file>` CLI command performs the import and reports what
  it mapped and what it could not place. A GUI button with a native file dialog
  calls the same code path.
- A per-source visibility toggle in the GUI. Local feedback is the first entry in
  the list and toggles identically to any imported source. Visibility is view
  state only and is not persisted.
- Positions the model cannot express are reported rather than silently dropped:
  an old-side (`original`) position and a whole-file position have no
  representation in a new-side line anchor, so they are counted and named in the
  import report.

## Capabilities

### New Capabilities
- `foreign-review-import`: the import format, its mapping onto the review model,
  comment provenance and immutability, the source registry, re-import
  idempotency, and the import report.
- `foreign-review-visibility`: the per-source visibility toggle, local feedback
  as an equal entry in that list, and how hidden sources affect the file-list
  status pills and the overview.

### Modified Capabilities
- `review-cli`: adds the `import` command, and requires the mutating commands to
  refuse an edit of imported comment text while still permitting reply, resolve,
  ignore and reactivate on it.
- `comment-reanchoring`: an imported comment's first anchor is built at import
  time from the current diff rather than from a reviewer's action, so the
  reconciler's existing rules apply to it unchanged. An imported comment whose
  line is absent from the diff starts adrift.

## Impact

- `backend/model/model.go`: an `Origin` field on `Comment`, a `Source` record and
  a source list on `Review`, and the guards that make an imported comment's text
  immutable. `Comment.UnmarshalJSON` already tolerates unknown-to-old-code
  fields in one direction; the new fields are additive and omitted when empty, so
  old state files load and newly written files stay readable by the CLI.
- New backend files for the import format's decoding and its mapping onto the
  model, kept separate from `model` so the format is a boundary rather than part
  of the domain.
- `backend/cli.go`: one command-table entry and its handler.
- `backend/main.go`: a bridge method for the GUI import, the file dialog, and the
  source list on the review info.
- `ts/core/types.ts`, `ts/core/comments.ts`: the `Origin` wire type, and the
  visibility filtering as pure functions over a comment list.
- `ts/render/comments.ts`: omit Edit and Delete for an imported comment.
- `ts/render/filelist.ts`, `ts/render/overview.ts`, `ts/render/state.ts`,
  `ts/main.ts`, `backend/assets/index.html`: the toggle control and its effect on
  the rendered surfaces.
- `1430.json` is committed as a test fixture, pinning the upstream tag names so a
  change to the upstream encoder fails a test here rather than at runtime.
- No new third-party dependency. No network access, no credentials.
