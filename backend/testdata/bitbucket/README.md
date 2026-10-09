# Test fixtures

Bitbucket Cloud REST 2.0 payloads for the `feedback/bitbucket` tests. Each comment
payload is a page envelope (`{values, page, pagelen, size, next}`), so the page
decoder and the pagination loop read the same data the decoders read.

Every file here is public or synthetic. Do not commit a recording of a private
repository: names, account identifiers and review text in it are not ours to
publish. Build a synthetic payload with the same structure instead.

## Files

| File | Source |
| --- | --- |
| `aui_5405_pr.json` | recorded: `GET /2.0/repositories/atlassian/aui/pullrequests/5405`, a public repository |
| `aui_5405_comments.json` | recorded: the above `/comments?pagelen=100&fields=+values.resolution.*` |
| `aui_5405_comments_page{1,2,3}.json` | recorded: the same collection at `pagelen=2`, with real `next` cursors |
| `synthetic_3754_{pr,comments}.json` | hand-built: deep reply chains, resolved threads, automated feedback |
| `synthetic_77_{pr,comments}.json` | hand-built: a payload that raises nearly every finding |
| `*.expected.json` | upstream `bb-feedback` output for the payload of the same name |
| `*.expected-findings.txt` | upstream `bb-feedback` findings for the payload of the same name |

## What each fixture covers

`aui_5405_comments.json`: an inline anchor with `to` set and `from` null, a reply
carrying `parent.id` and no anchor of its own, and two deleted comments whose
`content.raw` is `""`.

`aui_5405_comments_page{1,2,3}.json`: three recorded pages with recorded cursors.
Pages 1 and 2 carry `next` and page 3 does not.

`synthetic_3754_comments.json`:

- 25 comments, 12 from an `app_user` whose `kind` is `workspace_access_token` and 13 from three people
- 4 thread roots resolved with `{type, created_on, user}` detail
- a reply chain four levels deep under one root
- application authors with no `nickname`
- one deleted comment, so `comment_count` is 24

`synthetic_77_comments.json`: orphan replies, a reply cycle, a duplicate
identifier, an inverted range, an anchor with no path, an unknown author type, an
unexpanded resolution, a resolution on a reply, an unparseable timestamp, an
unknown pull request state, all four position cases, and a count mismatch.

## The count invariant

The comments collection's `size` counts deleted comments. The pull request's
`comment_count` does not. So `retrieved - deleted = comment_count`. This holds for
`atlassian/aui` 5405 (5 retrieved, 2 deleted, 3 reported) and for
`synthetic_3754` by construction.
