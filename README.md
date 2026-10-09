# Code Review Tool

Local-first code review tool that allows per-line annotations on diffs betweens current branch and the main/master branch.

Annotations are preserved between branches and revisions.

Uses Go and Wails for the backend; a TypeScript frontend built with Deno.

Final binary is small (<10MB) but with many system dependencies.

## Installation

Pre-requisites:

* WebKit2GTK (linux)
* WebKit (mac)
* [Deno](https://deno.com) (build-time only; not required to run the binary)


```bash
./manage.sh setup   # Checks WebKit and Deno, installs the Wails CLI
./manage.sh build
cp ./backend/build/bin/code-review /path/to/your/local/bin/

# or

./manage.sh release.install

# then

code-review
```

## Running

From inside a git repo:

    code-review

## CLI

Invoked with no arguments, `code-review` opens the GUI. Invoked with a command,
it exposes a read-only-on-the-repository CLI for acting on an in-progress review
— aimed at AI agents working in the project directory, on the branch under
review. It resolves the same review the GUI does (from the repository, current
branch, and default branch) and writes only the review state, never the repo.

    code-review --help          # list the commands
    code-review instructions    # the agent contract: how to use the CLI and review
    code-review start           # create the review for the current branch
    code-review list            # the active comments needing attention (JSON)
    code-review show <id>       # one comment with its reply thread
    code-review status          # summary: branches, comment counts, marked files
    code-review resolve <id>    # mark a comment addressed
    code-review reactivate <id> # set a resolved comment back to active
    code-review reply <id> <text>  # reply to a comment
    code-review comment <text>  # add a review-level (top-level) comment
    code-review unmark <file>   # drop a file's reviewed mark
    code-review import <file>   # merge an import document from a file
    code-review fetch [<url>]   # merge feedback from the branch's Bitbucket pull request
    code-review schema          # print the import document JSON Schema

### Fetching pull request feedback

`code-review fetch` with no argument finds the open Bitbucket Cloud pull request
whose source branch is the current branch, through the repository's `origin`
remote. `code-review fetch <url>` fetches the pull request at that URL. Both
merge its feedback into the review and print a report. `--debug` logs each
request to stderr, with the credential redacted.

Credentials are read from the environment only:

| Variable | Meaning |
| --- | --- |
| `BITBUCKET_ACCESS_TOKEN` | A repository, project or workspace access token, sent as a bearer token. Takes precedence. |
| `BITBUCKET_EMAIL` + `BITBUCKET_API_TOKEN` | An Atlassian account API token with the `read:pullrequest:bitbucket` scope, with the account email, sent as HTTP Basic. |
| neither | Anonymous, which is enough for a public repository. |

The GUI reads the same variables from the environment it was started in.

### Producing an import document

A program producing review feedback for `code-review import` codes against the
JSON Schema in `backend/feedback/import-document.v1.schema.json`, which
`code-review schema` also prints. Every document is validated against it before
import, and each violation is reported with its JSON Pointer. Properties the
schema does not define are rejected. One rule is enforced outside the schema: in
every span, `first` does not exceed `last`.

## Development

```bash
./manage.sh wails.dev    # Hot reload development mode
./manage.sh deno.bundle  # Rebuild the frontend bundle (assets/review.js)
./manage.sh deno.test    # Run the frontend (TypeScript) tests
./manage.sh test         # Run all tests (Go + frontend) with coverage
./manage.sh clean        # Remove build artifacts
```

