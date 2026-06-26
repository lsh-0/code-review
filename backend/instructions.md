This is the agent contract for the 'code-review' tool. It explains how to act on an in-progress code review using the `code-review` CLI. The review records reviewer feedback against a git diff (source_branch compared to target_branch) for the repository you are working in.

You act on the review entirely through the `code-review` command. Do not look for, read, or edit any state file by hand — the CLI is the interface, and it resolves the right review automatically from the repository and branch you are on. Run `code-review` in the repository, on the branch under review.

Commands:
- `code-review list` — the comments that need your attention (active root comments), each with an `id`, the `file` it sits on (empty for a review-level comment), its current `line`, an `outdated` flag, the `author`, and the `content`. This is where you find work.
- `code-review show <id>` — one comment with its full reply thread and current placement.
- `code-review status` — a summary of the review: the branches, the counts of comments by status, and the number of marked files.
- `code-review resolve <id>` — mark a comment addressed. Run this once you have made the change the comment asks for.
- `code-review reactivate <id>` — set a resolved comment back to active.
- `code-review reply <id> <text>` — add a reply to a comment's thread. This is your channel back to the reviewer: use it to record a blocker, a question, or what you decided and held back.
- `code-review comment <text>` — add a review-level (top-level) comment: overall feedback not anchored to any file.
- `code-review unmark <file>` — remove a file from the reviewer's "reviewed" set. Run this for every source file you modify.
- `code-review instructions` — print this contract.

The discovery flow is `list` → `show` → act: run `code-review list` to find the comments needing attention, `code-review show <id>` to read one and its thread, then make the change and `code-review resolve <id>`. `code-review instructions` is the front door — start here if you need to recall how the CLI works.

The `line` a comment reports is its last reconciled placement. When `outdated` is true the comment's code has moved and the line is no longer reliable; rely on the comment's content and surrounding context to locate the code, not the line number.

To act on the review: address every comment `code-review list` returns. Do the smaller, mechanical changes first, then the larger ones. A comment must be addressed unless it is genuinely impossible; if a comment seems impossible, you have probably misunderstood it, so re-read it (`code-review show <id>`) rather than skip it. Make the requested change in the actual source file, then `code-review resolve <id>`. Changing the status _is_ the feedback, so additional follow-up commentary is typically unnecessary.

A comment usually addresses a pattern, not only the lines it sits on. Feedback like "extract this into a convenience function" or "use the existing helper here" generally means apply that principle everywhere it holds across the project, not just at the commented location — search out the other sites and fix them too. This is what makes the change consistent and saves the same comment recurring next round.

Work unsupervised: do not pause to ask the reviewer for confirmation or print questions to the console — the reviewer does not see console output and will not answer mid-run. When a comment leaves a genuine judgement call (how widely to apply a change, a risky or far-reaching edit, two reasonable interpretations), make a decision and proceed; default to the smaller, safer change. Then record what you decided and what you held back with `code-review reply <id> <text>`, so the reviewer can direct the broader change in the next round. The reply thread is the only channel back to the reviewer, so use it for anything that needs their input rather than leaving the work undone.

If something blocks you from addressing a comment, leave it active and add a reply explaining the blocker with `code-review reply <id> <text>`, then let the reviewer decide. Do not try to dismiss or ignore a comment yourself.

Whenever you modify a source file while acting on this review, run `code-review unmark <file>` for that file, so the reviewer can see at a glance which reviewed files have changed and need revisiting. This applies to every file you edit, including files changed because feedback in one file applies to others that have no comments of their own.
