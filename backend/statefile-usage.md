This is a code-review state file for the 'code-review' tool. It records review comments against a git diff (source_branch compared to target_branch in repo_path).

Schema: `files` is an array of { file_path, comments[] }. Each comment has: `id` (stable identifier), `author`, `content` (the review note, in markdown), `line_number` (1-based line in the new version of the file), `status` (one of 'active', 'resolved', 'ignored'), and `context_before`/`context_line`/`context_after` (the surrounding source lines captured when the comment was made, used to relocate the comment if line numbers shift), and `replies` (an optional flat array of { id, author, content } child notes forming a thread under the comment; replies have no status of their own and only the root comment is resolvable).

To act on a review: address every comment with status 'active'. Do the smaller, mechanical changes first, then the larger ones. A comment must be addressed unless it is genuinely impossible; if a comment seems impossible, you have probably misunderstood it, so re-read it rather than skip it. Make the requested change in the actual source file at file_path within repo_path, then set that comment's `status` to 'resolved'.

Do not set `status` to 'ignored' on your own; leave such a comment 'active', add a reply explaining what blocked you, and let the reviewer decide.

You may append entries to a comment's `replies` array (each a { id, author, content } object with a new unique id); use replies to record a blocker, a question, or a note for the reviewer. Do not change `id`, `line_number`, or the context fields, do not edit or delete existing replies, and do not add or remove root comments.

`marked_files` is an array of file_path strings the reviewer has marked as reviewed. Whenever you modify a source file while acting on this review, remove that file's path from `marked_files` if present, so the reviewer can see at a glance which reviewed files have changed and need revisiting. This applies to every file you edit, including files changed because feedback in one file applies to others that have no comments of their own. Do not add paths to `marked_files`.

Apart from comment `status` values, appended replies, and removing entries from `marked_files`, preserve this `_readme` field and all other fields as-is. The file is JSON; write it back with the same structure.
