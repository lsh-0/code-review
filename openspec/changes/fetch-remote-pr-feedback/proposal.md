## Why

Importing hosted review feedback currently takes two programs and a file. The
OCaml `bb-feedback` program (`experiments/ocaml-lang`) fetches a Bitbucket pull
request and writes the normalised JSON document, and `code-review import <file>`
reads that document. The OCaml toolchain is not part of this project. Bitbucket
is also the first of several providers, so the provider-independent parts need a
home here. The import document has no machine-readable definition: its shape
exists only in upstream's encoder and this project's decoder. A program emitting
feedback for `code-review` has nothing to code against or validate with before
handing a document over.

## What Changes

- `bb-feedback` is ported to Go inside this project. The port covers the API
  client, cursor pagination with a loop guard, wire decoding, normalisation onto
  the existing import document, findings, and the `Validate` invariant checks
  including comment-count reconciliation.
- The import document types, their decoding, the findings and the validation
  checks move into a provider-independent package. Bitbucket-specific code goes in
  a provider package beneath it. The document's JSON encoding is unchanged, so
  `import <file>` still accepts `bb-feedback` output.
- A new `code-review fetch [url]` CLI command:
  - With no argument, it finds the open pull request for the current branch from
    the `origin` remote.
  - With a Bitbucket pull request URL, it fetches that pull request.
  - Either way it maps and merges the feedback through the existing import path
    and reports the outcome.
- Credentials come from environment variables only, as upstream:
  - `BITBUCKET_ACCESS_TOKEN` is sent as a bearer token.
  - Otherwise `BITBUCKET_EMAIL` and `BITBUCKET_API_TOKEN` are sent as HTTP Basic.
  - Otherwise requests are anonymous.
  - No credential is ever accepted as an argument or written to output.
- A GUI control runs the same fetch, either discovering the pull request or using
  a URL the reviewer pastes. Its report appears in the existing import banner.
- The import report gains the normalisation and validation findings at warning
  and error severity. It also gains a line when the pull request's source branch
  differs from the branch under review.
- A JSON Schema (draft 2020-12) for the import document is published. It is
  committed in the repository, embedded in the binary, and printed by a new
  `code-review schema` command.
- Every import document, whether from a file or from `fetch`, is validated
  against the schema before decoding. Violations are reported with their JSON
  Pointer location.
- An optional top-level `schema_version` is added. An absent value reads as 1,
  and any other value is rejected.
- **BREAKING:** properties the schema does not define are rejected. They were
  ignored before. Upstream `bb-feedback` output and `1430.json` carry none, so
  they still import.
- `import <file>` still makes no network request and reads no credential.

## Capabilities

### New Capabilities
- `remote-review-fetch`: retrieving review feedback from a hosted pull request.
  It covers locating the pull request from the git remote and branch or from a
  URL, credential resolution, read-only retrieval and pagination, normalisation
  onto the import document, findings and validation, and the error messages for
  each retrieval failure.

### Modified Capabilities
- `foreign-review-import`: the published schema becomes the normative
  definition of the import format. Documents are validated against it before
  decoding, unknown properties are rejected, and an optional `schema_version` is
  added.
- `review-cli`: adds the `fetch` and `schema` commands. Under `Conservative
  mutation surface` and `Repository-read-only operation`, `fetch` joins the
  mutating commands whose only write target is the state file.

## Impact

- New Go package `backend/feedback`. It holds the document types, decoding,
  encoding, findings and validation moved out of `backend/foreign_format.go`. It
  also holds `import-document.v1.schema.json`, embedded with `go:embed`.
- New Go package `backend/feedback/bitbucket`. It holds request URLs, wire types,
  credentials, retrieval over a substitutable HTTP client, and normalisation.
- `backend/foreign_import.go` and its tests now reference `feedback` types. The
  mapping and merge behaviour is unchanged.
- `backend/cli.go` has two new command-table entries and handlers. `backend/main.go`
  has a new bridge method. `backend/git.go` gains a read of the `origin` remote
  URL. `backend/instructions.md` documents both new commands and the schema.
- `backend/assets/index.html`, `ts/main.ts` and `ts/client.ts` add the fetch
  control.
- Recorded Bitbucket payloads from `experiments/ocaml-lang/test/fixtures` are
  committed under `backend/testdata/bitbucket/`.
- Network access is outbound HTTPS GET to `api.bitbucket.org`, from `fetch` only.
  It uses `net/http` from the standard library.
- One new third-party dependency is added:
  `github.com/santhosh-tekuri/jsonschema/v6`, a pure-Go 2020-12 validator. Its
  only runtime dependency is `golang.org/x/text`, which is already in `go.mod`.
  The standard library has no JSON Schema validator.
