## Context

The `code-review` GUI resolves an in-progress review's state file in `startup()`
from `cwd` → `GetGitRoot` → `GetCurrentBranch` / `GetDefaultBranch` →
`GetXDGDataDir` → `GetReviewStatePath`, then `LoadReview`/`SaveReview` operate on
that JSON. The agent contract currently lives in `backend/statefile-usage.md`,
embedded via `statefile.go` and stamped into every state file's `_readme` by
`SaveReview`. The GUI's `GetStatePrompt` produces the "copy AI prompt" text.

All resolution helpers are already package-level functions in package `main`, so
a CLI path in the same package can reuse them without refactoring. The state
mutators the CLI needs are pure methods on `model.Review` / `model.Comment`
(`Resolve`, `Reactivate`, `AddReply`, `AddComment`, `UnmarkFile`, `GetComment`,
`GetFileDiff`).

Comment placement is already tracked: each comment carries an ordered `Anchors`
history, and `model` exposes `CurrentLineNumber()` and `IsOutdated()` (an adrift
most-recent anchor). The GUI keeps anchors current via `ReanchorComments` on
refresh, so the persisted state already records each comment's current line and
whether it has gone adrift — the CLI reads these rather than recomputing.

## Goals / Non-Goals

**Goals:**

- Add subcommand dispatch to the binary without disturbing the GUI, version, or
  bare-invocation paths.
- Reuse the GUI's exact review-resolution chain so the CLI and GUI always agree
  on which state file is in play.
- Keep the repository strictly read-only; write only the state file.
- Emit a purpose-built JSON shape for read commands, decoupled from the
  `model` schema.
- Make `instructions` the single source of the agent contract; reduce `_readme`
  to a pointer.

**Non-Goals:**

- No new review-discipline prose is written now; `instructions.md` keeps the
  existing contract text (renamed), to be expanded in a later pass.
- No `ignore` command, no comment deletion, and no adding a *file-anchored* root
  comment (a file comment needs a line and captured context the CLI has no diff
  to derive). Adding a *review-level* (top-level, unattached) comment is in
  scope — it carries no line or context.
- No review-selection flags (`--source`/`--target`/`--state`).
- No change to the GUI's behaviour beyond the `_readme`/copy-prompt text.

## Decisions

### Dispatch ahead of `wails.Run`

`main()` inspects `os.Args` before constructing the Wails app. If the first
non-program argument is a recognised subcommand (or `-h`/`--help` listing them),
control routes to the CLI and the process exits with the CLI's status; otherwise
the existing GUI path runs. `--version` is preserved. This avoids pulling Wails
into CLI code paths.

A small dispatch table maps subcommand name → handler. `-h`/`--help`/unknown
commands print usage built from that table, so the command list has one
definition.

**Flag parsing: prefer `spf13/pflag`.** Per-subcommand flags use `pflag`
(`pflag.NewFlagSet` per command) for POSIX-style flags and clean composition
with subcommands; it is added as a direct dependency. The GUI path keeps its
existing `flag.Bool("version")` usage — the two coexist, since dispatch routes
to the CLI before the GUI's `flag.Parse`. The builtin `flag` is an acceptable
fallback if a `pflag` dependency is unwelcome, but `pflag` is the default.

### Two-layer resolution from the GUI's helpers

Resolution splits in two. `resolveTarget` derives the identity —
`{repoPath, sourceBranch, defaultBranch, userName, statePath}` — by calling the
same functions `startup()` uses, without touching the state file.
`resolveReview` builds on it: it requires the state file to exist and loads it
into a `reviewContext`. Failure cases map to clear, non-zero exits: not a repo,
or (for everything but `start`) no review for the branch, with the message
directing the reader to `code-review start`.

A command declares which it needs (`needsNothing` / `needsTarget` /
`needsReview`), and the dispatcher passes exactly that. `instructions` needs
nothing (runs outside a repo); `start` needs the target; all others need the
loaded review. This keeps each handler's signature honest about its input rather
than threading a half-built context.

The CLI does **not** recompute the diff or re-anchor — those stay GUI concerns.
Even `start` creates an *empty* review (no `RecomputeDiff`): the review's diff
and file list are populated when the GUI next opens it. The CLI reads the
persisted comments/marks as-is.

### `start` is the only creator

Only `start` writes a new state file, and only when none exists; it never
overwrites an existing review. It mirrors the GUI's creation sans diff:
`model.NewReview` + `os.MkdirAll` on the data dir (as `startup` does) +
`SaveReview`. Creation is read-only on the repository — no diff is taken — so it
does not weaken the read-only guarantee. Keeping the seeded review empty is a
deliberate first step: the near-term flow is human-guided (the reviewer seeds
comments in the GUI), and `start` exists so an agent can eventually bootstrap a
review unsupervised; listing the files to review is a future extension.

### Read-only guarantee

CLI commands call only the read helpers (`GetGitRoot`, `GetCurrentBranch`,
`GetDefaultBranch`, `GetUserName`) and `LoadReview`. Mutating commands then call
pure `model` methods and `SaveReview` against the state path. No CLI path calls
`RecomputeDiff`, `git checkout`, `git stash`, or any write-capable git command.
The spec's read-only requirement is satisfied by construction: the only write
syscall target is the state file.

### Purpose-built JSON shapes

Define CLI-local output structs (e.g. a flattened comment view: `id`, `file`,
`line`, `outdated`, `author`, `content`, `status`, `replies[]`) marshalled by
the command handlers, rather than serialising `model.Comment`. This insulates
agents from anchors/blobs and lets the internal schema evolve without breaking
the CLI contract. `line` is the comment's `CurrentLineNumber()`; `outdated` is
`IsOutdated()` (the comment's anchor has gone adrift, so its line is no longer
reliable); `file` is empty for review-level comments. `list` returns active root
comments; `show` returns one root with its thread; `status` returns branch +
counts + marked count.

### Comment lookup spans files and review-level

`resolve`/`reactivate`/`reply`/`show` take an id and must find it whether it is a
file comment or a review-level comment. A lookup helper scans `review.Files[*].
Comments` and `review.Comments`, returning the comment and its owning surface, so
`reply` can route to the right `AddReply`. Mutating a reply or unknown id is an
error (only root comments resolve/reactivate).

### Adding a review-level comment

A `comment <text>` command adds a review-level (top-level, unattached) comment
via `Review.AddComment`, authored as the git user, then `SaveReview`. This is
the one root-comment addition the CLI allows: a review-level comment carries no
line number or context, so the CLI can create it without a diff. The CLI does
**not** add file-anchored root comments — those need a line and a captured
context window the CLI has no diff to derive.

### `instructions.md` is the contract source — rewritten, not just renamed

`backend/statefile-usage.md` is renamed `backend/instructions.md`; the embed in
`statefile.go` (renamed variable, e.g. `instructionsText`) feeds both the
`instructions` command (prints it verbatim) and a new short `_readme` pointer.
`SaveReview` stamps the pointer into `_readme` instead of the full text.
`GetStatePrompt` returns the same pointer so the button and the file agree.

The existing prose describes the *state-file schema and how to hand-edit it*
(`status` values, `parent_id` threading, `marked_files` records, "write it back
with the same structure"). That is exactly what the CLI exists to hide, so the
state-manipulation mechanics are **removed and rewritten as CLI usage**. The
review *discipline* is principle, not mechanism, and is preserved. The mapping:

| Removed state-file mechanic | Replacement instruction |
|---|---|
| set a comment's `status` to `resolved` | `code-review resolve <id>` |
| append a reply with a new id / `parent_id` | `code-review reply <id> <text>` |
| remove a file's record from `marked_files` | `code-review unmark <file>` |
| leave a blocker `active` and reply | `code-review reply <id> <text>` (no `ignore` command exists) |
| top-level review feedback | `code-review comment <text>` |
| read the state file at `<path>`, follow `_readme` | `code-review list` / `code-review show <id>` to find work |
| schema description; "write it back with the same structure" | removed entirely — the agent never reads or writes the file |

Preserved verbatim (review discipline, not state mechanics): work unsupervised
(no console questions); a comment usually addresses a pattern, so apply it
everywhere it holds; address active comments, smaller/mechanical first; on a
judgement call make the smaller, safer decision and record what was held back as
a reply; do not treat a comment as impossible — re-read it.

The CLI-usage section also states the discovery flow (`list` → `show` → act) and
that `instructions` is the front door, so a fresh agent can bootstrap from this
one command. No new review-discipline content beyond the above is authored now.

The pointer is a small constant ("This review is managed by the `code-review`
tool. Run `code-review instructions` in this repository on this branch and
follow it."), defined once and reused by `SaveReview` and `GetStatePrompt`.

## Risks / Trade-offs

- **`_readme` no longer self-describing.** A reader without the binary on PATH
  loses the inline contract. Accepted: the CLI is the intended interface and the
  pointer names the exact command; the full text is one command away.
- **Line numbers reflect the last persisted anchor, not a fresh diff.** The CLI
  reads the placement the GUI last reconciled (`CurrentLineNumber`), and surfaces
  `IsOutdated` so the agent knows when a comment has gone adrift. It does not
  itself recompute the diff/re-anchor (that path shells out to a write-capable
  git surface and belongs to the GUI). After a new commit made outside the GUI,
  a `line` can lag until the GUI refreshes — but `outdated` flags the unreliable
  case, and mutations key on ids and file paths, not line numbers. Accepted.
- **Dispatch ordering vs `flag`.** Routing on `os.Args[1]` before `flag.Parse`
  must still honour `--version` and `-h`. Mitigated by handling these explicitly
  in the dispatch table and keeping the GUI's `flag` parse on the GUI path only.
- **Same-process self-write detection.** The GUI watches the state file's mtime;
  a CLI write is an external write the GUI will (correctly) surface as a refresh
  prompt — this is the intended async round-trip, not a regression.
