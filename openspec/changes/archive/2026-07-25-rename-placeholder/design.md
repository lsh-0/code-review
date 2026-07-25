## Context

The backend fetches a review's diff via `GetDiff` (`backend/git.go:42`), parses it
into `[]DiffFile` in `ParseDiff` (`backend/diff_parser.go`, `DiffFile` at
`backend/diff_parser.go:32`), and serves per-file metadata to the frontend through
`App.GetDiffFiles` (`backend/main.go:278`). The frontend mirrors the shape in
`ts/core/types.ts` (`DiffFile` at `:71`, `DiffFileMeta` at `:108`) and renders each
file's body in `ts/render/hunks.ts`, where the binary case already short-circuits to
a placeholder (`hunks.ts:59`).

A pure-rename file carries no content hunks, so its diff body is empty and reads as
a possible bug — the same failure mode the binary placeholder already solved. The
`diff-context-expansion` capability already owns the sibling "Binary files are
listed but not rendered" requirement, so the rename behaviour is modelled directly
on it: detect the condition while parsing, flag it on `DiffFile`, carry the flag to
the frontend, and render an explanatory placeholder instead of a blank pane.

Two things must hold for the placeholder to ever appear:

1. git must report the move as a rename. With `diff.renames` disabled, git emits an
   unrelated delete-plus-add and no `rename from` marker ever reaches the parser.
2. the parser must distinguish a pure rename (no hunks) from a rename with edits
   (rename header plus hunks), since only the former gets the placeholder.

The implementation for this change already exists in the worktree from an earlier
bundled lap; this design records the approach so the plan can be reviewed against
that code, not to direct fresh coding.

## Goals / Non-Goals

**Goals:**
- Force rename detection so a pure rename is reliably reported as a rename.
- Flag renamed files and record the pre-rename path, carried from the Go parser
  through to the frontend.
- Show an explanatory placeholder naming the old path for a pure rename, leaving a
  rename-with-edits to render its hunks normally.
- Mirror the binary-placeholder pattern so the two empty-pane cases are handled
  consistently.

**Non-Goals:**
- No change to how hunks, expansion affordances, comments, or navigation work for
  ordinary files or renames with edits.
- No handling of copy detection (`-C`) or rename-below-threshold cases; those
  remain reported as delete-plus-add and render as before.
- No preview of the moved file's content beyond the placeholder text.

## Decisions

**Force rename detection with `-M` rather than relying on `diff.renames`.** A repo
with rename detection off would otherwise show a pure rename as a delete plus an
add, and the placeholder branch would never be reached. Passing `-M` on the diff
command makes the behaviour independent of the user's config (`backend/git.go:42`).
Alternative considered: reading and honouring `diff.renames` — rejected because it
makes the feature silently depend on per-repo configuration the reviewer may not
control.

**Detect the rename from the `rename from` marker, mirroring `Binary`.** The parser
already keys off textual markers in the diff (`Binary files ... differ`); a
`rename from <path>` line is the same kind of signal. A new `renameFromRegex`
(`backend/diff_parser.go:47`) sets `Renamed` and captures `OldPath`
(`backend/diff_parser.go:77`), keeping the parser's structure uniform.
Alternative considered: inferring a rename from a `similarity index` line —
rejected because `rename from` is the marker that also yields the old path the
placeholder needs.

**Gate the placeholder on `Renamed && Hunks.length === 0`.** This is what separates
a pure rename from a rename with edits (`ts/render/hunks.ts:69`). A pure rename has
the marker and no hunks; a rename with edits has both and falls through to normal
rendering. Alternative
considered: a separate `PureRename` flag — rejected as redundant, since the hunk
count already distinguishes the two and avoids a second field that could disagree
with the hunks.

**Name the old path in the placeholder ("renamed from `<old>`, no changes") rather
than the bare "renamed, no changes" from the ticket.** The old path is the one
piece of information not otherwise visible in the UI once the file is shown under
its new name. The code falls back to the bare text when `OldPath` is empty. This is
flagged for the reviewer at the gate and is trivial to trim to the plain wording if
preferred.

**Keep `Renamed`/`OldPath` required on the TS types.** The project favours
drift-catching shape discipline: making the fields required forces every producer of
a `DiffFile`/`DiffFileMeta` to supply them, so a missing field is a type error
rather than a silent `undefined` at render time.

## Risks / Trade-offs

- [The placeholder pins an exact UI string, which can drift if the copy is later
  reworded] → `render_dom_test.ts` asserts the text, so a reword breaks the test and
  forces a matching spec update; requirement and test move together.
- [`-M` adds rename-detection work to every diff, not only renames] → git's rename
  detection is cheap relative to producing the diff itself, and it runs once per
  review load; the correctness gain (reliable detection independent of config)
  outweighs it.
- [Recording a decision the code already satisfies risks looking like a no-op] →
  accepted deliberately: the value is the durable contract and the review gate over
  the already-written code, so a future change cannot silently revert the rename
  pane to a blank, unexplained body.

## Open Questions

- Placeholder wording: name the old path, or use the plain "renamed, no changes"
  from the ticket? Defaulted to naming the old path; the reviewer decides at the
  gate.
