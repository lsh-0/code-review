## Context

The previous change (`2026-09-15-import-foreign-review-feedback`) added file
import of the upstream `Code_feedback` JSON document. Its pieces are these:

- `backend/foreign_format.go` holds the document's Go types and a validating
  decoder in `package main`.
- `backend/foreign_import.go` holds the pure mapping and merge.
- `cmdImport` and `App.ImportForeignReview` are the two entry points. Each
  computes placement inputs and calls `importForeignReview`.

Upstream `bb-feedback` is about 1,800 lines of OCaml outside its tests, in these
layers:

- `bitbucket_api`: request URLs.
- `http_client`: credentials, errors and status classification.
- `fetch`: GET and pagination over a client functor.
- `bitbucket_wire`: decoders for the API payload.
- `normalise`: wire to `Code_feedback`, plus findings.
- `validate`: invariant checks over `Code_feedback`.
- `render_json`: encoding.
- `bin/main.ml`: the shell.

Its tests replay recorded payloads from `test/fixtures`. The built binary runs
offline with `--pr-file` and `--comments-file`, so it can produce golden
documents for parity tests.

The backend has no logging today. Errors are returned and printed by `runCLI`.

## Goals / Non-Goals

**Goals:**
- Behavioural parity with upstream on the recorded fixtures, so the port is
  checked against the program it replaces.
- A provider-independent core: document, findings and validation. A second
  provider adds only a package that fetches and normalises onto it.
- One fetch path shared by the CLI and the GUI. That path ends in the existing
  `importForeignReview`.
- Retrieval testable without a network, through a substitutable HTTP client.
- A machine-readable contract for the import document that any emitter can
  validate against before handing a document to `code-review`.

**Non-Goals:**
- GitHub or any provider other than Bitbucket Cloud.
- Writing anything to the provider: no comment posting and no resolution sync.
- A credential store, a GUI credential prompt, or a token file. The source is the
  environment only.
- Keeping `experiments/ocaml-lang` in sync after the port. It becomes reference
  material.
- Porting `bb-feedback`'s `--pr-file`/`--comments-file` replay as a CLI flag.
  Replay exists in tests only.

## Decisions

### 1. Package layout: a provider-independent core and one package per provider

```
backend/feedback/             provider-independent
  import-document.v1.schema.json   the contract, embedded with go:embed
  schema.go                   Schema() []byte, Validate([]byte) error
  document.go                 Document, Thread, Comment, Position, ... (moved from foreign_format.go)
  decode.go                   schema-validating decoder, Encode for parity tests
  finding.go                  Severity, Code, Finding, Sort, MaxSeverity
  validate.go                 Check(Document) []Finding
backend/feedback/bitbucket/   Bitbucket Cloud only
  url.go                      ParsePullRequestURL, ParseRemote, request URLs
  credentials.go              Resolve(getenv) (Credentials, []string)
  client.go                   Doer interface, Get, Classify, error kinds
  fetch.go                    PullRequest, Comments (pagination), FindOpenForBranch
  wire.go                     API payload types and decoding
  normalise.go                Normalise(pr, comments) (feedback.Document, []feedback.Finding)
```

`package main` keeps the mapping and merge in `foreign_import.go`, the git reads
and the two entry points.

The previous change kept the format in `package main` because nothing else
consumed it. Now two producers exist: file decode and Bitbucket normalisation.
`Validate` is explicitly provider-independent and must not import a provider
package. A package boundary enforces that direction at compile time.

Considered: keeping everything in `package main` with a `bb_` file prefix. That
was rejected because it gives no import-direction guarantee. A second provider
could then reach into Bitbucket internals with nothing to stop it.

Moving the types renames them, for example `foreignThread` to `feedback.Thread`.
The existing tests in `package main` follow the rename. The JSON tags are
unchanged, so `1430.json` and existing state files are unaffected.

### 2. The IO boundary is a one-method interface

```go
// sends one request. Implemented by `*http.Client` in production and by a
// recorded-response replayer in tests.
type Doer interface {
    Do(*http.Request) (*http.Response, error)
}
```

This mirrors upstream's `Http_client.S` functor. `*http.Client` already satisfies
it, so production needs no adapter. Tests supply a replayer keyed by URL that
serves the committed fixtures, so the pagination loop under test is the one that
runs in production.

The production client is `&http.Client{Timeout: 30 * time.Second}` with the
default transport, which verifies against the system trust store. No
`InsecureSkipVerify` exists anywhere. Context cancellation is threaded through
`http.NewRequestWithContext`. The GUI passes the Wails context, so closing the
window cancels a fetch in flight.

### 3. Errors are a closed set of kinds, with messages ported verbatim

```go
type ErrorKind int
const (Transport ErrorKind = iota; Unauthorized; NoBitbucketScopes; NotFound; UnexpectedStatus; CursorLoop; Undecodable)
type Error struct { Kind ErrorKind; Status int; Detail string }
```

`Classify(status, body)` ports `Http_client.classify`, including the
`no Bitbucket scopes` body sniff. `Error()` ports `pp_error`'s remedy text.
`Detail` holds a response body or a URL and never a request header, so no code
path can place a credential in an error. Tests use `errors.As` and check `Kind`.

### 4. Credentials: a sum type with redaction as the only rendering

```go
type Credentials struct { kind credKind; email, token string }  // unexported fields
func (c Credentials) Redacted() string       // "access token (<redacted>)"
func (c Credentials) String() string         // = Redacted(), so %v cannot leak
func (c Credentials) authorize(*http.Request)
```

`Resolve(getenv func(string) string)` is a pure port of `Credentials.resolve`.
`os.Getenv` is passed only at the entry points. Unexported fields with a
redacting `String` and `GoString` mean `fmt` cannot print the secret. Tests
assert that `fmt.Sprintf("%v %+v %#v", c, c, c)` does not contain the token.

Upstream's credential leak came from dependency logging. This port has no
logging dependency. `net/http` does not log headers. Debug logging (decision 10)
logs the URL and `Redacted()` only.

A GUI launched from a desktop launcher inherits the session environment and not a
shell's. A token exported only in `.bashrc` is therefore absent and the request is
anonymous. That is the accepted consequence of the env-only decision. The GUI
reports a 404 or 401 with the same remedy text as the CLI, which names the
variables.

### 5. Discovery reads `origin` and queries open pull requests by source branch

`git remote get-url origin` is read-only and joins `backend/git.go`.
`ParseRemote` accepts `git@bitbucket.org:ws/repo(.git)`,
`ssh://git@bitbucket.org/ws/repo(.git)` and `https://[user@]bitbucket.org/ws/repo(.git)`.

The query is
`GET /2.0/repositories/{ws}/{repo}/pullrequests?state=OPEN&q=source.branch.name="<branch>"`,
with `q` percent-encoded and the branch's `"` and `\` escaped. It follows `next`
cursors with the same loop guard as the comments collection. Exactly one result
is used. Several results produce an error listing each number and its
`links.html.href`.

Considered: picking the most recently updated of several matches. That was
rejected because a guess merges feedback from the wrong pull request into a
review, and re-import then removes it again. The URL form resolves the case
deterministically.

### 6. Parity is asserted against upstream golden output

For each recorded fixture pair (`aui_5405`, `synthetic_3754`, and the
`aui_5405_comments_page{1,2,3}` pagination set), the upstream binary is run once,
offline. Its stdout is committed as
`backend/testdata/bitbucket/<name>.expected.json` and its findings report as
`<name>.expected-findings.txt`. A Go test normalises the same payloads, encodes
with `feedback.Encode`, and compares the result after a JSON round trip to
`any`, so key order and whitespace do not matter. A second assertion compares
finding codes and severities.

This pins behaviour to the program being replaced and not to a re-reading of it.
Timestamps are the subtle part. Upstream renders UTC with exactly six fractional
digits and a `Z` suffix. The Go encoder uses the layout
`2006-01-02T15:04:05.000000Z`, and parity fails if they diverge.

`feedback.Encode` exists for this test and for the fuzz round trip. The fetch
path passes the `Document` value to the mapping directly and does not round-trip
through JSON.

### 7. The JSON Schema is the contract, and the decoder sits behind it

There are two validation layers, and they answer different questions. Schema
validation asks whether a document is a well-formed import document. Its answer
is shared with every emitter. `feedback.Check` asks whether the feedback is
internally consistent, for example a reply cycle or a count mismatch. That answer
is a finding and does not reject the document. Only the first is a contract.

**The schema shape.** It is one file, `import-document.v1.schema.json`, using
`$schema: https://json-schema.org/draft/2020-12/schema` and a `$id` under the
project's repository URL. It has `$defs` for `span`, `position`, `location`,
`body`, `author`, `resolution`, `comment` (recursive through `replies`),
`thread`, `revision`, `participant` and `pull_request`. The definitions are
derived from `render_json.ml`, because that is the encoding in practice:

- Each tagged union declares its tag as an `enum` and every property any of its
  tags may carry, then sets `additionalProperties: false`. A list of `if`/`then`
  rules, one per tag, requires that tag's properties and sets the others to
  `false`. For example `whole_file` forbids `lines`, and `unchanged` requires
  both spans.
- This layout was chosen over a `oneOf` of branches for the errors it produces.
  `oneOf` reports one failure per branch, so an unknown tag never lists the
  allowed values in one place. An `unevaluatedProperties` variant reports every
  sibling and parent property as not allowed whenever a nested value fails. With
  the chosen layout, an unknown tag is one violation listing the allowed values.
  A misspelt key is always reported, independent of any other violation on the
  same object.
- Optionals that upstream writes as `null` (`nickname`, `account_id`, `uuid`,
  `description`, `updated_at`, `commit`) are `type: ["string", "null"]` and
  are not required.
- Timestamps use `format: date-time`, with format assertion enabled in the
  validator. Upstream always writes RFC 3339, so this is a check and not a
  guess.
- `display_name` and markdown `text` use `minLength: 1` with the pattern
  `[^ \t\n\r\f]`. The explicit class is the whitespace upstream trims, and it
  means the same in RE2 and ECMA regex. `\S` differs between those dialects.
  This is the existing decoder rule ("never empty", blank rejected) moved into
  the contract.
- `span.first` and `span.last` are integers with `minimum: 1`. `first <= last`
  cannot be expressed, so it is stated in `description` and enforced in Go.
- `schema_version` is `{"const": 1}` and is not required.

**Enforcement.** `feedback.Decode(data)` runs these steps in order:
1. `json.Unmarshal` into `any`, to reject invalid JSON.
2. A `schema_version` pre-check, so a version 2 document produces "unsupported
   version 2, supported: 1" and not a wall of `const` violations.
3. Schema validation, with every violation collected.
4. `json.Unmarshal` into the typed `Document`.
5. The span-ordering check.

The hand-written tag and emptiness checks in `validateForeignThread` are deleted,
because the schema now owns them. Step 5 is the one rule kept in Go.

Error rendering flattens the validator's `*ValidationError` tree to its leaf
causes. Each becomes a line `<instance pointer>: <message>`, sorted by pointer
for a deterministic order. Existing tests assert that the error names the field,
for example `position.kind`. Those assertions change to the pointer form, for
example `/file_feedback/0/location/position/kind`. The names are equivalent and
precise.

**Validator choice.** The standard library has no validator, so a dependency is
unavoidable for runtime validation. `github.com/santhosh-tekuri/jsonschema/v6`
has these properties:
- full 2020-12 support, passing the official JSON-Schema-Test-Suite
- `format` assertion
- structured errors carrying both instance and schema locations
- a single runtime dependency, `golang.org/x/text`, already present indirectly
  through Wails

The schema is compiled once from the embedded bytes, through `sync.OnceValue`. A
compile failure is a programming error and is caught by a unit test.

Considered: `github.com/google/jsonschema-go`. It has a smaller API and weaker
2020-12 coverage. Its errors carry less location detail, and location detail is
the thing an emitter needs.

Considered: test-time conformance only. That was rejected because it leaves the
hand-written decoder as the source of truth and the schema as documentation that
can drift.

**Publishing.** The committed file lives at
`backend/feedback/import-document.v1.schema.json`, because `go:embed` cannot
reach outside the module directory. `code-review schema` prints the embedded
bytes. `instructions.md` and the README reference both. A test asserts that the
printed bytes equal the file.

**Fetch goes through the same gate.** The fetch path encodes the normalised
`Document` with `feedback.Encode`, then calls `feedback.Decode` on the bytes. A
normaliser bug that produces a non-conforming document then fails exactly as an
external emitter's would. The round trip costs microseconds per pull request.

**Versioning.** Version 1 is the current upstream encoding. A future breaking
change adds `import-document.v2.schema.json`, and `Decode` selects a schema by
`schema_version`. This change does not build the dispatch: one version exists,
and the pre-check rejects others.

### 8. The pull request type gains the fields upstream already writes

`Check` needs `reported_comment_count`, participants and `updated_at`. The current
Go `foreignPullRequest` omits several of these because the import ignored them.
The moved `feedback.PullRequest` adds the missing fields with their upstream tags.
The schema requires the fields an importer cannot default: identifiers, tags,
authors, bodies, `created_at`, branches and the thread structure. Nullable fields,
`pending`, `replies`, `participants`, the counts and `draft` are optional, so a
producer writes only what it knows. Upstream writes all of them, and its output
validates. The encoding is unchanged. The Go struct simply stops discarding fields upstream already writes.

`feedback.Check` runs on the fetch path. It does not run on the
`import <file>` path, which gains schema validation only. Making `import` report
findings is a follow-up.

### 9. One fetch function, two thin entry points

```go
// pure apart from `doer` and `git`: locate → retrieve → normalise → validate.
func fetchForeignDocument(ctx context.Context, doer bitbucket.Doer, creds bitbucket.Credentials,
    target fetchTarget) (feedback.Document, []feedback.Finding, error)
```

`fetchTarget` is either a parsed URL or `{remoteURL, sourceBranch}` for discovery.
`cmdFetch` and `App.FetchForeignReview(url string)` each resolve credentials from
`os.Getenv`, call this function, then run the same
`importPlacement` → `importForeignReview` → persist sequence as their import
counterparts.

The report type gains two optional parts:
- `Findings []feedback.Finding`, rendered as one line each at warning and error.
- A branch-mismatch line when `doc.PullRequest.Source.Branch` differs from
  `review.SourceBranch`.

Nothing is persisted until retrieval and normalisation succeed. That is what
leaves state unchanged on any retrieval failure.

`foreignSourceID` already derives `bb:<number>` from the document. Fetched and
file-imported documents for the same pull request therefore share a source
without further work.

### 10. Logging: `log/slog` to stderr, INFO by default, scoped to fetch

`fetch` is the first long-running, networked operation, and progress belongs on
stderr. `cmdFetch` builds a `slog.TextHandler` on `errOut` at INFO. A
`--debug` flag on `fetch` lowers it to DEBUG. The levels are:

- INFO: which pull request is fetched and with which redacted credential kind,
  and how many comments were retrieved.
- DEBUG: each request URL with the redacted kind, and debug-severity findings.
- WARN: credential-resolution warnings.

The report lines remain on stdout.

The GUI passes a handler writing to the process stderr at INFO. Its user-facing
outcome goes through the banner.

The rest of the backend keeps its current error-return style. Adopting `slog`
everywhere is outside this change.

### 11. The GUI control: one button, one optional URL field

A `Fetch feedback` button sits beside `Import feedback`, with a small text input
for an optional URL. An empty input means discovery. The button is disabled while
the bridge call is in flight. The outcome reuses `showImportReport` and the
post-import reload in `triggerImport`. A bridge error appears in the same banner.
No modal is added.

Considered: a modal dialog with a URL field. That was rejected as heavier than
the case needs. The common case is the empty input.

## Risks / Trade-offs

- **Rejecting unknown properties breaks a lenient emitter.** → Upstream output
  and `1430.json` carry no extra properties. Both are tested. An emitter that
  wants to attach its own data needs a schema change, and that is the point of
  having one.
- **The schema and the Go types drift.** → The golden documents, `1430.json`
  and every hand-built test document are validated against the schema. A fuzz
  target asserts that any document the schema accepts decodes into `Document`,
  and that `Encode` of any decoded `Document` is schema-valid.
- **A third-party validator in the import path.** → It is pure Go, has one
  dependency already present, and its version is pinned in `go.sum`. The schema
  is a plain file, so replacing the library does not change the contract.

- **The port diverges from upstream in an untested corner.** → Golden parity on
  three recorded fixtures, plus ports of upstream's hand-built normalisation
  tables: the 16-combination position table, cycles, orphans and duplicates.
  `Original`, `Unchanged` and multi-line ranges have never appeared in live data
  upstream either, so they rely on the ported hand-built tests.
- **Bitbucket's `q` filter syntax or pagination changes.** → Discovery failures
  surface as errors with the remedy "give a URL". Comments retrieval does not
  depend on `q`.
- **A GUI launched outside a shell sees no token.** → Accepted. The error text
  names the environment variables, and public repositories still work.
- **Moving the document types is a wide rename in tests.** → It is mechanical,
  and the JSON tags do not change. The existing decoder, mapping, merge and fuzz
  tests run unchanged in behaviour, which is the check that the move preserved
  semantics.
- **An error finding (for example a reply cycle) still merges.** → This matches
  upstream, which writes the document and exits 1. The findings are shown in the
  report and every comment is preserved. Refusing would discard usable feedback
  for one malformed thread.
- **Network access enters a tool that was offline.** → It is confined to one
  command and one bridge method, GET only, to `api.bitbucket.org` only. The
  host is fixed in the URL builder and is never taken from a cursor's host
  without a check. `next` cursors are accepted only when their scheme and host
  match the API base.

## Migration Plan

No state migration is needed. The document encoding and the state file shape are
unchanged. A document that imported before this change and carries an undefined
property now fails with an error naming that property. Removing the property
restores the import. Rollback consists of removing the `fetch` command, the bridge method
and the control. Feedback already fetched remains as an ordinary imported source.
