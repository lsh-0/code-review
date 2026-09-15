## Context

The upstream producer already did the normalisation. `experiments/ocaml-lang`
(`bb-feedback`) fetches a Bitbucket pull request and writes a JSON document whose
types are documented as provider-independent in `lib/code_feedback.mli`:

> Independent of Bitbucket: nothing here mentions the wire format, and a second
> source could be normalised onto these types.

So this change does not design a general-purpose shape. It mirrors one that
exists, is encoded by a single module (`lib/render_json.ml`), and is covered
upstream by a round-trip property test. `1430.json` is a sample of that encoding:
one `pull_request`, one review-level thread, six file-anchored threads, all
single-line `updated` positions with empty reply lists.

On this side, the review model is a `Review` holding `Files []*FileDiff` (each a
path plus a flat `[]*Comment`) and a review-level `Comments []*Comment`. A comment
is anchored through `Anchors []Anchor`, and two derived properties drive
everything downstream:

```go
func (a Anchor) IsAdrift() bool { return len(a.Context) == 0 }
func (c *Comment) IsOutdated() bool { ... anchor != nil && anchor.IsAdrift() }
```

This is the constraint that shapes the import. A comment built with
`NewComment(content, 69, author)` gets `Anchors: [{LineNumber: 69}]` — no context,
therefore adrift, therefore outdated, therefore filtered out of
`getCommentsForLine` and rendered in the untethered block with an empty
pseudo-hunk. A naive import would put every comment in that block with nothing to
show. The import must capture a real context window, or accept adrift deliberately.

The state file is read by agents and by the CLI, and modifying its shape has wide
consequences: `surfaces()`, `findComment`, `GetComments`, `GetCommentedFiles`,
`GetReviewComments`, `ReanchorComments`, `mutationResult` and the frontend's
`commentsCache` all assume the current arrangement. The brief is explicit that
the correct move here is the least impactful one.

## Goals / Non-Goals

**Goals:**
- Import the upstream document into the existing comment lists, additively, so
  every existing state file remains valid with no migration step.
- Preserve imported reply threads, including nesting deeper than the model's one
  level.
- Imported comment text immutable; reply, resolve, ignore and reactivate all work.
- Re-import is idempotent and preserves local replies and local status.
- One import code path shared by the CLI and the GUI.
- Per-source visibility in the GUI, with local feedback as an equal entry.
- Report what could not be placed rather than dropping it silently.

**Non-Goals:**
- Any network access, credential handling, or provider API client. The tool reads
  a file. Everything provider-specific stays upstream.
- Writing anything back to a provider. The import is one-way.
- Retaining the source document verbatim in the state file. The model captures
  what it can use; the rest is metadata with no consumer, and the document itself
  is reproducible by re-running the upstream program.
- Importing pull request metadata beyond what the source registry needs for a
  label: no title rendering, no description, no reviewer approval state.
- Restructuring the state file to hold local feedback as one source among many.
  The uniformity that the toggle needs is achievable in view state.

## Decisions

### 1. Provenance is a field on `Comment`, not a parallel comment tree

An `Origin` on `Comment`, empty for a local comment:

```go
// where a comment came from. An empty Origin means the comment was written
// locally in this tool. An imported comment names the source it came from and
// its identifier there, which is what makes a re-import idempotent.
type Origin struct {
    Source    string `json:"source"`
    ForeignID string `json:"foreign_id"`
}
```

`omitempty` on the field keeps it off local comments, so an existing state file
loads unchanged and a newly written one stays readable by an older binary.

The alternative — a separate `Review.Foreign` tree merged at read time — is
structurally stronger on immutability and worse everywhere else. It needs its own
reanchoring pass or a merged one, its own status computation in both Go and
TypeScript, and a read path that synthesizes `*model.Comment` values which the
write path must not mutate. `findComment` already returns a pointer into the
review, so that last one is a bug class with no existing defence. Reanchoring,
`FileCommentStatus`, `GetCommentedFiles`, the mutation result and the whole
frontend render path all keep working untouched with a field on `Comment`, and
that is what "display it like regular feedback" means in practice.

The cost is accepted openly: immutability becomes a guard at each mutation site
rather than a guarantee from the type. There are exactly two sites in Go
(`UpdateComment`, `DeleteComment` on the bridge) and the CLI exposes neither, so
the guard surface is small and testable.

### 2. Nested replies flatten by re-rooting, reusing the existing rule

The upstream `comment` type is recursive (`replies : comment list`) and the
reference pull request threads four levels deep. The model's threads are flat.

`appendReply` already solves this for the local case:

```go
root := parentID
if parent := findComment(comments, parentID); parent != nil && parent.ParentID != "" {
    root = parent.ParentID
}
```

A reply to a reply re-roots to the same root. Import walks the reply tree
depth-first and gives every descendant the root comment's ID as `ParentID`. That
preserves every comment, its author, and its order, and it applies the one
flattening rule the tool already has rather than inventing a second.

What is lost is the tree structure: a reply that answered another reply reads as
answering the root. Preserving depth would mean a recursive `ParentID` chain, and
`getReplies`/`createCommentElement` render exactly one level of nesting, so it
would mean changing the render layer as well as the model. Comments are preserved;
their reply-to relationships beyond the first level are not.

### 3. Place blind at the stated line, capturing a real window from the current diff

Import sets `LineNumber` from the position's span start and captures a
`captureRadius`-sized window from the recomputed diff at that line, producing a
good first anchor with the current blob SHA. The first `reanchorComment` pass then
short-circuits on `current.Blob == blob` and the comment behaves exactly as one
created locally at that line now.

```
  document: config/server.conf, updated lines {first:69, last:69}
        │
        │  look up line 69 in diffLinesByPath(a.diffFiles)
        │  capture ±captureRadius, read current blob SHA
        ▼
  Anchor{Blob: <sha@HEAD>, LineNumber: 69, Offset: 3, Context: [7 lines]}
        │
        │  existing reconciler, unchanged, from here on
        ▼
  tracks the code as the branch advances
```

Line 69 absent from the diff yields `Anchor{Blob: <sha@HEAD>}` — adrift, so the
comment renders in the existing untethered block, which is the honest outcome.

The accepted risk: if the file changed since the pull request was reviewed, line
69 now holds different code, and the import captures that different code as the
comment's context. Reanchoring will then faithfully track the wrong lines. The
document does carry `pull_request.source.commit` (`8ebf8c861e7f` in the sample),
so a later change could read the file at that revision and let the reconciler walk
the comment forward from there. That is strictly more accurate and strictly more
work, needs a fetchable commit, and needs its own fallback for when the commit is
absent. Blind placement was chosen for this change; the field stays in the format
for later.

Where the frontend captures a window it uses `captureContextRadius` in
`ts/core/overview.ts`, and the backend uses `captureRadius` in
`backend/model/reanchor.go`, with comments on both requiring they stay equal.
Import is a third capture site and uses the backend constant, so the invariant
becomes two-of-three in Go and one in TypeScript.

### 4. The four position cases map to two outcomes

```
  Code_feedback.position       →  model.Anchor
  ─────────────────────────       ────────────────────────────────────
  Updated(span)                →  LineNumber = span.first, capture window
  Unchanged{orig, upd}         →  LineNumber = upd.first,  capture window
  Original(span)               →  adrift; old-side lines have no anchor
  Whole_file                   →  adrift; addresses no line at all
```

The last two import as untethered comments on their file rather than being
rejected. They are real feedback and the untethered block is exactly the surface
for a comment that cannot be placed. The import report counts them separately so
the reviewer is told, rather than wondering why a comment appears at the top of a
file with no context.

Rejecting them was considered and declined: it silently loses feedback the reviewer
asked for, and the rendering surface for the case already exists.

### 5. The format is a boundary, decoded outside `model`

The decoding lives in its own backend file (a `foreignreview` package or a
`foreign_*.go` pair in `backend`), not in `model`. `model` stays the domain and
gains only `Origin` and `Source`. The mapping from the decoded document to
`*model.Comment` values is a pure function of the document plus the current diff
lines and blob SHAs, so it is testable against the committed `1430.json` fixture
with no git and no filesystem — the same discipline `ReanchorComments` and
`EvictChangedMarks` already follow.

Tagged unions decode by tag name into Go structs, with an unrecognised tag
returning an error. Upstream is explicit about why defaulting is wrong:

> Deliberately not folded into [Human]: defaulting an unrecognised author to a
> person would misreport exactly the distinction this type exists to draw.

The same reasoning applies to position kinds, where a wrong default would place a
comment on the wrong side of the diff. `1430.json` is committed as a fixture and a
decoder test pins every tag name, so a change to `render_json.ml` upstream fails a
test here rather than at runtime.

### 6. Source identity and the registry

```go
type Source struct {
    ID         string `json:"id"`          // "bb:1430"
    Label      string `json:"label"`       // "Bitbucket #1430 — Telemetry flag wiring"
    PullRequest int   `json:"pull_request"`
    SourceBranch string `json:"source_branch"`
    TargetBranch string `json:"target_branch"`
    ImportedAt string `json:"imported_at"`
}
```

`Review.Sources []Source` with `omitempty`, so a review with no imports is
byte-identical on disk to today.

The ID is derived from the document, not from the filename: a provider prefix and
the pull request number. Two pull requests from different repositories could
collide under that scheme, which is a real limitation and is accepted for now —
the tool reviews one branch of one repository, so two documents from different
repositories landing in one review indicates a mistake worth noticing rather than a
case worth supporting. The recorded source and target branches let a reviewer see
that an import does not match the branches under review.

Re-import matches on `(Source, ForeignID)`:

- present in both: keep the existing comment's `ID` and `Status`, so local replies
  stay attached and local resolution is not reverted. Content is replaced from the
  document, which is the only way an edited-at-source comment updates.
- present locally, absent in the document: remove, and remove its local replies
  with it, since the thread no longer exists at the source.
- absent locally, present in the document: add.

Keeping the local `ID` is what makes local replies survive: a reply's `ParentID`
points at that ID, so regenerating it would orphan the reply.

### 7. Visibility is view state, with local as element zero of the view model

The toggle needs a uniform list. It does not need the state file to provide one.

```
  state file (unchanged arrangement)      view state (ts/render/state.ts)
  ──────────────────────────────────      ──────────────────────────────
  Review.Files[].Comments  ─┐             ┌──────────────────────────┐
  Review.Comments          ─┤             │ [0] Local            [x] │
  Review.Sources[]         ─┘   ──────▶   │ [1] Bitbucket #1430  [x] │
    each comment carries Origin           └──────────────────────────┘
    ("" = local)                            hiddenSources: Set<string>
```

Local feedback is element zero of the rendered list and toggles by the same
mechanism as any import, which is what you asked for. It is not element zero of
`Review`, because moving local comments into a `Sources[0]` container touches every
consumer named in the Context and changes the format agents read, for no gain the
view model does not already give.

`hiddenSources: Set<string>` joins `fileFilter` and `fileGrouping` in `ViewState`
— both in-session, both unpersisted, both already precedents for exactly this. The
filtering is a pure predicate in `ts/core/comments.ts` over a comment list, applied
where `commentsFor(path)` is read.

A local reply is hidden with the thread it belongs to, not with local feedback:
the predicate resolves a reply's visibility through its root's origin. A reply
rendered without the comment it answers has no meaning.

### 8. File-list pills already compute in the frontend

`ts/render/filelist.ts:27` derives the pill from the cached comments via the pure
`fileCommentStatus`, so filtering the input list gives correct pills for the
visible subset with no new ownership question.

The one exception is `ts/render/mutate.ts:69`, which sets the pill from
`result.file_status` computed by Go. Go does not know what is hidden, so after a
mutation the pill would briefly reflect all sources. The fix is local: `mutate.ts`
recomputes from the patched cache instead of using the supplied field. The Go
field stays in the mutation result, unused by that one call site, rather than
changing the bridge shape.

### 9. CLI first, GUI button over the same code

`code-review import <file>` is one entry in the `commands` table and one handler,
using the existing `needsReview` dispatch and `resolveReview`. The GUI button
needs `runtime.OpenFileDialog` (Wails runtime is currently used only for
`EventsEmit` in `watcher.go`), a bridge method, and frontend wiring — all of it
over the same mapping function.

CLI first because the agent is a first-class user: an agent that can run
`code-review import ./1430.json` pulls the feedback in without the GUI. It also
means the import is covered by ordinary CLI tests before any dialog exists.

The command is a mutation, so `Conservative mutation surface` in the `review-cli`
spec needs updating: the list of mutating commands is normative there.

## Risks / Trade-offs

- **Blind placement can anchor a comment to unrelated code.** → Reported: the
  import states how many comments were placed and how many are untethered. The
  document carries `source.commit`, so the accurate variant remains available
  without a format change.
- **Immutability is a guard, not a type guarantee.** → Two Go mutation sites, both
  tested, and the CLI exposes no edit or delete command at all. A future mutation
  site that forgets the guard is a real regression risk.
- **Reply nesting beyond one level is lost.** → Every comment, author and order is
  preserved; only the reply-to-a-reply relationship flattens. It applies the same
  rule the tool already applies locally.
- **Upstream format drift breaks the import silently in principle.** → `1430.json`
  committed as a fixture, tag names pinned by a decoder test, unknown tags erroring
  rather than defaulting. Drift fails a test.
- **A document from the wrong branch imports comments that render nowhere.**
  `GetCommentedFiles` iterates `a.diffFiles`, so a `FileDiff` for an absent path is
  invisible. → The report names unmatched paths and calls out the wholly unmatched
  case. The comments are still in the state file and visible to the CLI.
- **Source IDs can collide across repositories.** → Accepted. The recorded branches
  make a mismatched import visible.
- **A third context-capture site to keep in lockstep.** → Import uses the backend's
  `captureRadius` directly rather than a copy, so the two Go sites cannot diverge.
- **Re-import replaces content, so a locally-read comment can change under the
  reviewer.** → That is the point of re-importing, and it is the only way an
  edited-at-source comment updates. Local replies and local status survive.

## Migration Plan

No migration step. Every added field is additive with `omitempty`:

- An existing state file loads with no `Sources` and no `Origin` on any comment, so
  every comment is local and behaves exactly as before.
- A state file written after this change, with no imports, is byte-identical to one
  written before it.
- A state file with imports, read by a binary from before this change, loses the
  origins and the source list on its next write, which would make imported comments
  indistinguishable from local ones and editable. That is a downgrade hazard, not a
  migration, and matches how the existing `MarkedFiles` and legacy-anchor
  tolerances are one-directional.

Rollback is removing the import command and the toggle. Comments already imported
stay in the state file as ordinary comments.

## Resolved Questions

- **An unmatched document imports and reports; it does not refuse.** Refusing was
  considered as a guard against importing into the wrong branch, and declined:
  reporting is more forgiving when only some paths match, and the comments remain
  reachable through the CLI either way. The report names each unmatched path and
  calls out the wholly-unmatched case as likely belonging to another branch.
- **The source registry does not record the document's path.** A path on disk is not
  app state: it goes stale, it is meaningless on another machine, and a state file
  that carries one invites a reader to trust it. The source is identified by the
  pull request it came from, derived from the document itself, so re-importing the
  same pull request from a moved or renamed file still matches its comments.
