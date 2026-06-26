# Tasks

## 1. Instructions source and `_readme` pointer

- [x] 1.1 Rename `backend/statefile-usage.md` to `backend/instructions.md`.
- [x] 1.2 Rewrite the state-manipulation mechanics in that file as CLI usage: replace "set `status`" → `resolve <id>`, "append a reply with `parent_id`" → `reply <id> <text>`, "remove from `marked_files`" → `unmark <file>`, "leave active + reply on a blocker" → `reply <id> <text>`, top-level feedback → `comment <text>`, and "read the state file / follow `_readme`" → `list` then `show <id>` to find work. Remove the schema description and "write it back with the same structure".
- [x] 1.3 Preserve the review-discipline prose verbatim (work unsupervised, apply-a-pattern-everywhere, smallest-first, smaller/safer decision + record held-back work as a reply, re-read rather than skip). Add a short discovery-flow note (`list` → `show` → act) and that `instructions` is the front door.
- [x] 1.4 Update the `//go:embed` directive and variable name in `backend/statefile.go` (e.g. `statefileUsage` → `instructionsText`) to point at `instructions.md`.
- [x] 1.5 Define a single `_readme` pointer constant directing the reader to run `code-review instructions` on the current repo/branch.
- [x] 1.6 Change `SaveReview` (`backend/storage.go`) to stamp the pointer constant into `Review.Readme` instead of the full embedded text.
- [x] 1.7 Change the GUI's `GetStatePrompt` (`backend/main.go`) to return the same pointer.

## 2. CLI scaffolding and dispatch

- [x] 2.1 Add `github.com/spf13/pflag` to `backend/go.mod` (`go get`); use a `pflag.FlagSet` per subcommand for flag parsing.
- [x] 2.2 Add a CLI entry path (new file, e.g. `backend/cli.go`) with a dispatch table mapping subcommand name → handler and a short usage line each.
- [x] 2.3 In `main()`, before constructing the Wails app, inspect `os.Args`: route `-h`/`--help` to usage, recognised subcommands to the CLI, preserve `--version`, and fall through to the GUI for bare invocation. Exit with the CLI's status when it handles the call.
- [x] 2.4 Print usage (built from the dispatch table) for `-h`/`--help` and for an unknown subcommand; unknown exits non-zero.

## 3. Review resolution for the CLI

- [x] 3.1 Add `resolveTarget` that derives `{repoPath, sourceBranch, defaultBranch, userName, statePath}` from `cwd` (read-only, no state-file touch), and `resolveReview` that requires the file to exist and loads it into a `reviewContext`.
- [x] 3.2 Map failures to clear non-zero exits: not a git repository; no state file for the current branch (directing the reader to `start`). Do not call `RecomputeDiff`.
- [x] 3.3 Dispatch by a per-command `needs` level (`needsNothing`/`needsTarget`/`needsReview`) so each handler receives exactly its required context.

## 4. Comment lookup helper

- [x] 4.1 Add a helper that finds a comment by id across `review.Files[*].Comments` and `review.Comments`, returning the comment and its owning surface (file path or review-level).

## 5. Read commands (JSON output)

- [x] 5.1 Define CLI-local output structs: a flattened comment view (`id`, `file`, `line` from `CurrentLineNumber()`, `outdated` from `IsOutdated()`, `author`, `content`, `status`, `replies[]`) and a status summary, independent of `model`.
- [x] 5.2 Implement `list`: emit active root comments (file and review-level) as a JSON array; empty array when none.
- [x] 5.3 Implement `show <id>`: emit one root comment with its ordered reply thread; non-zero error on unknown id.
- [x] 5.4 Implement `status`: emit source/target branches, active/resolved/ignored root counts, and marked-file count.

## 6. Mutating commands (state-file writes only)

- [x] 6.1 Implement `resolve <id>`: set a root comment resolved via `model`, `SaveReview`; error (no write) on reply target or unknown id.
- [x] 6.2 Implement `reactivate <id>`: set a resolved root comment active, `SaveReview`; same error handling.
- [x] 6.3 Implement `reply <id> <text>`: route to file or review-level `AddReply` as the git user, `SaveReview`; error (no write) on unknown id; parent status unchanged.
- [x] 6.4 Implement `unmark <file>`: `Review.UnmarkFile` + `SaveReview`; unmarking an unmarked file succeeds with the marked set unchanged.
- [x] 6.5 Implement `comment <text>`: add a review-level comment via `Review.AddComment` as the git user, `SaveReview`; error (no write) on empty text.

## 7. Instructions and start commands

- [x] 7.1 Implement `instructions`: print the embedded `instructionsText` verbatim to stdout, exit zero.
- [x] 7.2 Implement `start`: create the state file for the current branch via `resolveTarget` + `model.NewReview` + `os.MkdirAll` (data dir) + `SaveReview`; no-op report if a review already exists; never recompute the diff. Read-only on the repository.

## 8. Tests

- [x] 8.1 Unit-test the comment-lookup helper across file and review-level comments and the unknown-id case.
- [x] 8.2 Unit-test the JSON output shapes (`list`, `show`, `status`) against a fixture review, asserting internal anchor/blob fields are absent and that `outdated` reflects an adrift anchor.
- [x] 8.3 Unit-test each mutator's effect on an in-memory review (`resolve`, `reactivate`, `reply`, `unmark`, `comment`), including the no-op unmark, empty-text `comment`, and error-on-reply/unknown cases.
- [x] 8.4 Test that resolution errors (no repo, no review) yield non-zero exits with clear messages.
- [x] 8.6 Test `start`: creates the review when absent (then `list` succeeds), is a no-op that preserves feedback when one exists, and errors outside a repository.
- [x] 8.5 Assert the read-only guarantee: a mutating command writes only the state path (e.g. the repo's git status is unchanged across a command run in a fixture repo).
- [x] 8.7 Test the JSON output contract directly: each read command's stdout is valid JSON.
- [x] 8.8 Test POSIX-style flag parsing: an unknown subcommand flag exits 2, and a `--` terminator passes a dashed token through to the handler as a positional.

## 9. Documentation

- [x] 9.1a Document the CLI in README.md (Running/CLI section).
- [ ] 9.1b Update CHANGELOG via the changelog skill — deferred to the user (skill-managed).
- [ ] 9.2 Mark the TODO item "Add a code-review CLI for agents instead of hand-editing JSON" as addressed — deferred to the user (todo skill-managed).
