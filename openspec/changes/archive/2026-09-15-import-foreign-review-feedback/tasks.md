## 1. Model: provenance and the source registry

- [x] 1.1 In `backend/model/model.go`, add an `Origin` struct (`Source`,
  `ForeignID`, both `omitempty`) and an `Origin` field on `Comment` with
  `omitempty`. Add `IsForeign()` reporting a non-empty `Source`. Add the field to
  `Comment.UnmarshalJSON`'s `wire` struct so the legacy-upgrade path preserves it.
  Implemented as `*Origin`: `omitempty` does not suppress a zero struct, so a value
  type would write `"origin":{}` onto every local comment. Added `SourceID()`
  alongside `IsForeign()` so callers group by provenance without unpacking the
  pointer.
- [x] 1.2 Add a `Source` struct (id, label, pull request number, source and target
  branch, imported-at) and `Review.Sources []Source` with `omitempty`. Add
  `Review.GetSource(id)` and `Review.UpsertSource(Source)`.
- [x] 1.3 In `backend/model/model_test.go`, cover: a state file with no origins and
  no sources loads with every comment local and `Sources` empty; a review with no
  imports marshals without an origin or sources field; a comment with an origin
  round-trips; `UpsertSource` replaces by id rather than appending. Also covers the
  legacy-anchor upgrade path preserving an origin, since both forms share one
  `wire` struct.

## 2. Import format decoding

- [x] 2.1 Commit `1430.json` as a test fixture under `backend/testdata/`, as
  `foreign_review_1430.json`.
- [x] 2.2 Add a new backend file for the import format's Go types, mirroring the
  upstream `Code_feedback` JSON encoding: document (`pull_request`, `discussion`,
  `file_feedback`), thread (`location`, `resolution`, `root`), comment (`id`,
  `author`, `body`, `created_at`, `updated_at`, `pending`, `replies`), and the
  tagged unions for location, position, body, author kind and resolution state.
  Keep it out of `model` — it is a boundary format, not the domain.
- [x] 2.3 Implement decoding that resolves each tagged union by its tag name and
  returns an error naming the tag and its field for an unrecognised value. No
  defaulting: an unknown position kind or author kind is an error.
- [x] 2.4 Test decoding against the `1430.json` fixture: the document decodes; the
  one review-level thread and six file-anchored threads are recognised by their
  `location.kind`; the author kind `application` with its `application_kind` is
  read; every position is a single-line `updated` span. Pin every tag name by
  asserting an unrecognised value in each union errors.
- [x] 2.5 Test the cases the fixture does not exercise, with hand-built documents:
  a multi-line span, an `unchanged` position with both spans, an `original`
  position, a `whole_file` position, a `deleted` body, a `pending` comment, a
  thread with replies nested three levels deep, a `resolved` resolution, and a
  thread whose `location` contradicts the array it appears in.
- [x] 2.6 Test rejection: invalid JSON, a missing required field, a span with
  `first > last`, and each unrecognised tag, all returning an error.

## 3. Mapping the document onto the review

- [x] 3.1 Write the mapping as a pure function taking the decoded document, the
  new-side diff lines per path (`model.DiffLineRef`, as `diffLinesByPath` already
  produces), and the current blob SHAs per path; returning the comments to merge
  plus a report. No git, no filesystem, so it is testable against the fixture
  alone.
- [x] 3.2 Place a new-side position at its span's first line: look the line up in
  the diff lines, capture a window using the backend's existing `captureRadius`
  (import from `model`, do not copy the constant), and build a good first anchor
  with the current blob SHA and the anchored line's offset. `captureRadius` and
  `captureWindow` are unexported in `model`, so this added `model.AnchorAtLine`
  there rather than copying the constant into `package main`.
- [x] 3.3 Map an `unchanged` position through its updated span. Map an `original` or
  `whole_file` position, and any line absent from the diff lines, to an adrift
  first anchor carrying only the current blob SHA.
- [x] 3.4 Flatten reply trees by re-rooting: walk each thread depth-first and give
  every descendant the root's ID as `ParentID`, preserving order and per-comment
  author. Give each imported comment an `Origin` carrying the source id and the
  document's comment id as a string.
- [x] 3.5 Skip comments whose body is deleted or that are marked pending, counting
  each. Skipping a root skips its whole thread.
- [x] 3.6 Build the report: comments imported, placed, untethered, skipped, and the
  unmatched file paths (a path in the document with no entry in the diff lines),
  including the wholly-unmatched case. Verification found the unplaceable case was
  counted but its file never named, which the spec requires ("name each affected
  file"). Replaced the `Unplaceable` counter with an `UnplaceablePaths` list.
- [x] 3.7 Derive the source record: id as a provider prefix plus the pull request
  number, a label from the number and title, the source and target branch names,
  and the import time. Verification found the pull request title survived only
  folded into the label, which the spec scenario lists as its own item; added a
  `Title` field to `model.Source`.
- [x] 3.8 Test the mapping against the fixture: six file-anchored comments and one
  review-level comment; each carrying an origin; a comment on a line present in the
  supplied diff lines gets a good anchor at that line with a captured window; a
  comment on an absent line is adrift. Test the nested-reply flattening preserves
  every comment, its order and its author, with no comment parented to a reply.

## 4. Merging and re-import idempotency

- [x] 4.1 Write the merge as a pure function over the review, a source id, and the
  mapped comments: match existing imported comments by `(Source, ForeignID)`;
  for a match keep the existing comment's `ID` and `Status` and replace its content
  and anchors; add unmatched incoming comments; remove imported comments of that
  source absent from the incoming set, together with their local replies. A root
  adopting its stored id must carry its incoming replies with it: they were built
  against the id the mapping generated, so the merge re-parents them or they orphan.
- [x] 4.2 Ensure the merge only ever touches comments whose origin names this
  source: local comments and other sources' comments are untouched.
- [x] 4.3 Upsert the source record.
- [x] 4.4 Test: importing the same document twice yields one copy of each comment;
  a local reply added between imports stays attached; a locally-resolved imported
  comment stays resolved; a comment absent from the second document is removed with
  its local replies; importing a second source leaves the first intact; local
  comments survive every re-import.

## 5. Immutability guards

- [x] 5.1 In `backend/main.go`, make `UpdateComment` and `DeleteComment` return an
  error naming the reason when the target comment has a non-empty origin, before
  any mutation or persist.
- [x] 5.2 Confirm reply, resolve, ignore and reactivate work unchanged on an
  imported comment, and that a local reply to an imported comment is itself
  editable and deletable.
- [x] 5.3 Test both refusals leave the state file unmodified, and that the four
  permitted actions succeed on an imported comment.

## 6. CLI import command

- [x] 6.1 Add `import` to the `commands` table in `backend/cli.go` with
  `needs: needsReview` and a `cmdImport` handler taking one positional path
  argument. Read the file, decode, map against the current diff lines and blob
  SHAs, merge, then persist.
- [x] 6.2 Print the report to stdout: the counts, and any unmatched paths named. A
  read, decode, or mapping failure returns an error and leaves the state file
  untouched.
- [x] 6.3 Expose the origin's source in `commentView` as an `origin` field
  (`omitempty`), so `list` and `show` tell an agent a comment's text is not
  editable. Do not expose the foreign identifier. Added to `replyView` too, so a
  local reply under an imported root is distinguishable in a `show`.
- [x] 6.4 In `backend/cli_test.go`, cover: a successful import writes the state file
  and prints the counts; a missing path errors non-zero with the state file
  unchanged; a malformed document errors non-zero with the state file unchanged;
  wrong argument count prints usage; `list` includes an active imported comment
  with its origin; `status` counts imported comments; `resolve` and `reply` work on
  an imported comment id.
- [x] 6.5 Update `backend/instructions.md` so the agent contract covers the
  `import` command, that an imported comment's text cannot be edited, and that
  reply/resolve/reactivate work on one.

## 7. GUI import

- [x] 7.1 Add an `ImportForeignReview` bridge method on `App` that opens a native
  file dialog via the Wails runtime, then runs the same decode-map-merge-persist
  path as the CLI, returning the report as JSON.
- [x] 7.2 Add the import button to `backend/assets/index.html` beside the existing
  refresh and copy-prompt controls, wire it in `ts/main.ts`, and surface the report
  to the reviewer (counts, and unmatched paths when any). The report gets its own
  dismissable banner: the working-tree banner is owned by the working-tree watcher,
  which rewrites it on every poll.
- [x] 7.3 After a successful import, reload review state and re-render the current
  surface so the imported feedback appears without a manual refresh.

## 8. Wire types and provenance in the frontend

- [x] 8.1 In `ts/core/types.ts`, add the `Origin` interface and the optional
  `origin` field on `Comment`. Add the `Source` interface, and the source list to
  the review-info shape.
- [x] 8.2 In `backend/fixtures_test.go`, extend the wire fixtures to include a
  review with an imported comment and a source, so the TypeScript wire-type tests
  pin the new shape.
- [x] 8.3 In `ts/render/comments.ts`, omit the Edit and Delete buttons for a comment
  with a non-empty origin, and label an imported comment's source in its header.
  Keep Reply and the status actions.
- [x] 8.4 Test the render: an imported root comment renders with Reply and the
  status actions but no Edit or Delete; a local reply under it renders with Edit and
  Delete.

## 9. Source visibility

- [x] 9.1 In `ts/core/comments.ts`, add pure visibility functions: given a comment
  list and the set of hidden source ids, return the visible comments. A comment is
  hidden when its own source is hidden; a reply's visibility follows its root's
  source, so a local reply to a visible imported comment stays visible and a local
  reply to a hidden one is hidden. Local feedback is a source id of its own.
- [x] 9.2 Test the predicate: a local root hidden with local feedback; an imported
  root hidden with its source; a local reply to a hidden imported root hidden; a
  local reply to a visible imported root visible; nothing hidden when the hidden set
  is empty.
- [x] 9.3 In `ts/render/state.ts`, add `hiddenSources: Set<string>` and a `sources`
  list to `ViewState`, both in-session like `fileFilter` and `fileGrouping`. Reset
  both in `ts/render/dom_test_setup.ts`.
- [x] 9.4 Apply the visibility filter where comments are read for rendering, so it
  covers the line threads, the untethered block, the overview's per-file and
  review-level sections, and the file-list status pills. Do not render the
  untethered block when no visible comment is untethered. Applied inside
  `commentsFor` in `ts/render/state.ts`, the one accessor every render surface
  already reads through, rather than at each call site. `allCommentsFor` exposes the
  unfiltered list for the cache-patching and lookup paths that must not filter.
- [x] 9.5 In `ts/render/mutate.ts`, recompute the file status pill from the patched
  visible cache instead of using the Go-supplied `file_status`, so a mutation does
  not restore a pill for hidden sources. Leave the bridge shape unchanged.

## 10. Visibility control

- [x] 10.1 Add the source-toggle control to `backend/assets/index.html` beside the
  grouping and filter controls, with the styling in `backend/assets/style.css`.
- [x] 10.2 In `ts/main.ts`, build the toggle list from the review's sources with
  local feedback as the first entry, every entry checked by default, and re-render
  the file list and current surface on a change. Follow `setupGroupControl` as the
  precedent. Verification found the entry list was untestable inside `main.ts`, which
  has no test file; extracted it as the pure `sourceToggles` in `core/comments.ts`,
  which `renderSourceToggles` now consumes.
- [x] 10.3 Test the toggle list and its effects. The entry list is covered by six
  `sourceToggles` unit tests (ordering, labels, per-entry visibility, empty when
  nothing is imported); the rendered effects are covered by the DOM tests. Note the
  spec scenario "a review with no imports shows one entry" was wrong and has been
  corrected: the control is hidden with nothing imported, since a lone toggle that
  hides every comment is a trap rather than a choice. Persistence is asserted from the
  Go side instead of by stubbing the bridge: `TestStateFileCarriesNoVisibilityState`
  proves no visibility state reaches the state file.

## 11. Property and fuzz testing

- [x] 11.0 Add fuzz targets over the import path in `backend/foreign_fuzz_test.go`:
  the decoder must not panic on any input and must not accept a document violating
  the invariants it claims to check; the mapping's invariants (provenance, unique
  ids, one-level threads, report partitioning, sorted path lists) must hold for any
  accepted document; and a merge must be idempotent with local comments untouched.
  Add a `./manage.sh fuzz [duration]` target so this is reachable without recalling
  the flags. Fuzzing found a real defect: a `markdown` body with no `text` field
  decoded successfully and produced a comment with empty content, which would render
  as an unactionable blank thread. Upstream declares that text, and an author's
  display name, never empty; both are now rejected at the boundary. The crashing
  input is committed under `backend/testdata/fuzz/` as a regression test.

## 11. Verification

- [x] 11.1 Run `./manage.sh` test and lint targets for both Go and TypeScript, and
  confirm the whole suite passes.
- [x] 11.2 Import `1430.json` into a review in a repository where some of its paths
  exist, and confirm the report's counts, that placed comments render against their
  lines, that untethered comments render in the untethered block, and that the
  unmatched paths are named. Verified: 7 imported, 2 placed (lines 69 and 22, the two
  paths that exist), 4 untethered, 3 unmatched paths named. The placed anchor carries
  the real blob SHA, offset 3, and a 7-line window with the anchored line at that
  offset.
- [x] 11.3 Confirm end to end in the GUI: an imported comment offers Reply but not
  Edit or Delete; a reply to it persists and is itself editable; resolving it works;
  toggling its source hides it and its reply from the diff pane, the overview and
  the file pills; restarting shows every source visible again. Covered by the DOM
  tests in `ts/render/foreign_dom_test.ts` (action set, source label, hiding from the
  diff pane and the pills) and the guard tests in `backend/foreign_guard_test.go`.
  Not verified by driving the windowed GUI itself, which needs a display.
- [x] 11.4 Confirm the CLI round trip: `code-review import`, then `list` showing the
  imported comments with their origin, `reply` and `resolve` against one, then a
  re-import preserving that reply and that resolution. Verified against a real
  repository: every listed comment reported `origin: bb:1430`, the local reply
  reported no origin at all, and after re-import the comment was still resolved with
  its one reply attached.
- [x] 11.5 Update `CHANGELOG.md`, and remove the now-completed "Import review
  feedback from Bitbucket and GitHub" entry from `TODO.md`.
