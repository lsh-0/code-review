## Why

Today an AI agent acting on a review must read and hand-edit the state JSON
directly, guided only by the `_readme` prose stamped into that file. Editing
JSON by hand is error-prone (malformed status values, broken reply threading,
orphaned ids) and couples the agent to an internal schema that the GUI owns. A
direct CLI gives agents a small set of safe, validated actions and a single
front door for instructions, removing the need to know the state file exists.

## What Changes

- The `code-review` binary becomes dual-mode: bare invocation opens the GUI as
  it does today; invoking with a subcommand (revealed by `-h`/`--help`) runs a
  read-only-on-the-repo CLI against the in-progress review.
- The CLI resolves which review to act on exactly as the GUI does — from the
  current directory's git root, the current branch, the default branch, and the
  XDG data dir (`~/.local/share/code-review`). No flags select the review.
- The CLI performs **read-only git operations only**. It must never switch
  branches, stash, commit, revert, or otherwise modify the repository; its only
  writes are to the state file.
- Read commands: `list` (comments needing attention), `show <id>` (one comment
  with its thread), `status` (review summary). Output is purpose-built JSON by
  default — never the raw state-file schema.
- `start` creates the review state file for the current branch when none exists
  (read-only on the repo, never overwriting an existing review). It is the only
  command that creates a review; every other command requires one to exist and
  directs the reader to `start` otherwise. The review it creates is empty for now
  (no diff or file list) — a deliberate first step toward unsupervised agent
  reviews, kept minimal while the human-guided flow is proven.
- Mutating commands, kept deliberately conservative: `resolve <id>`,
  `reactivate <id>`, `reply <id> <text>`, `unmark <file>`, and `comment <text>`
  (a review-level/top-level comment, unattached to any file). No `ignore`, no
  comment deletion, and no adding file-anchored root comments — matching the
  existing agent policy.
- Read commands surface each comment's current line and an `outdated` flag
  (derived from the existing anchor history / adrift detection), so an agent
  knows when a placement is no longer reliable. The CLI reads the GUI's last
  reconciled anchors; it does not recompute the diff.
- Per-subcommand flags use `spf13/pflag` (a new direct dependency) for
  POSIX-style flags and subcommand composition; the GUI's existing `flag` usage
  is preserved.
- `instructions` becomes the agent's front door: it prints the agent contract
  (how to use the CLI and how to conduct the review). `backend/statefile-usage.md`
  is renamed to `backend/instructions.md` and its embedded contents back this
  command.
- **BREAKING** (internal): the state file's `_readme` field stops carrying the
  full agent contract. It is replaced by a short pointer instructing the reader
  to run `code-review instructions`. The "copy AI prompt" button emits the same
  pointer.

## Capabilities

### New Capabilities

- `review-cli`: the agent-facing command-line interface to an in-progress
  review — subcommand dispatch, repo-read-only review resolution, the read and
  mutate commands, JSON output contract, and the `instructions` front door.

### Modified Capabilities

<!-- No existing spec capabilities; openspec/specs/ is empty. The _readme and
     copy-prompt changes are covered as impact of the new capability. -->

## Impact

- `backend/main.go`: `main()` gains subcommand dispatch ahead of `wails.Run`;
  bare/`--version`/GUI paths preserved. New CLI command handlers.
- `backend/go.mod`: add `github.com/spf13/pflag` as a direct dependency for the
  CLI's flag parsing.
- `backend/statefile.go` / `backend/statefile-usage.md`: file renamed to
  `backend/instructions.md`; the embed feeds the `instructions` command.
- `backend/storage.go` + `model`: state load/save reused; `_readme` text
  replaced with the short pointer.
- Review-resolution helpers (`GetGitRoot`, `GetCurrentBranch`,
  `GetDefaultBranch`, `GetReviewStatePath`) are reused for the CLI; no GUI
  behaviour changes.
- The GUI's `GetStatePrompt` (the "copy AI prompt" text) changes to point at
  `code-review instructions`.
