## Why

A pure-rename file — one git detects as a rename with 100% similarity, so it carries
no content hunks — renders a blank diff pane with nothing to explain it. Like the
empty pane shown for a binary file, this reads as a bug: the reviewer cannot tell
whether the file is unchanged, failed to load, or simply has no textual diff. The
binary case was given an explanatory placeholder; the rename case is its direct
sibling and should be handled the same way.

## What Changes

- Pure renames SHALL be detected reliably. `GetDiff` passes `-M` so git reports a
  rename as a rename (`rename from`/`rename to`) regardless of the user's
  `diff.renames` config, rather than as an unrelated delete-plus-add pair that
  would never surface a rename.
- The diff parser SHALL flag a renamed file and record its pre-rename path. The
  `DiffFile` shape gains `Renamed` and `OldPath`, carried through the diff-file
  metadata to the frontend, mirroring how `Binary` is detected and propagated.
- Selecting a pure rename SHALL show an explanatory placeholder naming the old
  path ("renamed from `<old>`, no changes") instead of a blank pane, with no hunks.
- A rename WITH edits (above git's similarity threshold) is unaffected: it keeps
  its hunks and renders normally. A rename below the threshold (reported as
  delete-plus-add) is also unaffected.

No breaking change: the new fields are additive, and existing diff rendering is
untouched except for the new pure-rename branch.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `diff-context-expansion`: gains a new requirement covering rename detection and
  the pure-rename placeholder. This capability already owns the sibling "Binary
  files are listed but not rendered" requirement, so the rename behaviour belongs
  here alongside it rather than in a separate capability.

## Impact

- `backend/git.go` — `GetDiff` passes `-M` to force rename detection.
- `backend/diff_parser.go` — `DiffFile` gains `Renamed`/`OldPath`; a `rename from`
  marker sets them.
- `backend/main.go` — `GetDiffFiles` metadata carries `Renamed`/`OldPath`.
- `ts/core/types.ts` — `DiffFile` and `DiffFileMeta` gain the two fields.
- `ts/render/hunks.ts` — renders the `.rename-placeholder` for a pure rename.
- `ts/render/load.ts`, `ts/render/state.ts` — carry the fields through state and
  the lazy fetch.
- `backend/assets/style.css` — `.rename-placeholder` shares `.binary-placeholder`
  styling.
- Tests: `backend/diff_parser_test.go`, `backend/git_test.go`,
  `ts/render/render_dom_test.ts`.

Note for the reviewer: the implementation for this change is already present in the
worktree (committed during an earlier bundled lap, before this planning artifact
existed). This proposal records the change so the plan can be reviewed against the
code at the review gate; it does not introduce new code.
