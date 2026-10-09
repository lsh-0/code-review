## Purpose

Retrieve review feedback from a pull request hosted with a provider and turn it
into the provider-independent import document. The local review then merges it
through the existing import path, without a separate program or an intermediate
file. Bitbucket Cloud is the first provider.

## ADDED Requirements

### Requirement: A pull request is located from a URL

The system SHALL accept a Bitbucket Cloud pull request URL of the form
`https://bitbucket.org/<workspace>/<repository>/pull-requests/<number>`. Trailing
path segments, a query and a fragment SHALL be accepted. The workspace,
repository and number SHALL be read from the URL. A URL that does not name all
three, or whose host is not a supported provider, SHALL be rejected with an error
that quotes the URL.

#### Scenario: An overview URL is accepted

- **WHEN** the URL is `https://bitbucket.org/acme/router/pull-requests/1430/overview`
- **THEN** the pull request is workspace `acme`, repository `router`, number `1430`

#### Scenario: A URL without a number is rejected

- **WHEN** the URL is `https://bitbucket.org/acme/router/pull-requests/`
- **THEN** an error quoting the URL is returned and no request is issued

#### Scenario: An unsupported host is rejected

- **WHEN** the URL's host is not `bitbucket.org`
- **THEN** an error naming the host as unsupported is returned and no request is issued

### Requirement: A pull request is discovered from the current branch

With no URL, the system SHALL read the repository's `origin` remote URL and derive
the workspace and repository from it. It SHALL accept the SSH (`git@host:` and
`ssh://`) forms and the HTTPS form. It SHALL then request the open pull requests
whose source branch is the review's source branch. Exactly one match SHALL be
used. Zero matches and multiple matches SHALL each be an error.

#### Scenario: One open pull request for the branch

- **WHEN** `origin` is `git@bitbucket.org:acme/router.git` and one open pull request has source branch `feature-x`
- **AND** the review's source branch is `feature-x`
- **THEN** that pull request is fetched

#### Scenario: No open pull request for the branch

- **WHEN** no open pull request has the review's source branch
- **THEN** an error naming the branch and the repository is returned

#### Scenario: Several open pull requests for the branch

- **WHEN** more than one open pull request has the review's source branch
- **THEN** an error listing each match's number and URL is returned
- **AND** the error states that a URL selects one

#### Scenario: The remote is not a supported provider

- **WHEN** `origin` is absent, or its host is not a supported provider
- **THEN** an error stating that discovery needs a supported `origin` remote is returned, and a URL can be given instead

### Requirement: Credentials come from the environment only

The system SHALL resolve credentials from the environment:
- `BITBUCKET_ACCESS_TOKEN` is sent as a bearer token and takes precedence.
- `BITBUCKET_EMAIL` with `BITBUCKET_API_TOKEN` is sent as HTTP Basic.
- With neither, requests SHALL be anonymous.

A credential SHALL NOT be accepted as an argument or through the GUI.

#### Scenario: Access token takes precedence

- **WHEN** `BITBUCKET_ACCESS_TOKEN` and both Basic variables are set
- **THEN** requests carry a bearer token
- **AND** a warning states that the Basic variables were ignored

#### Scenario: A partial Basic pair is ignored

- **WHEN** only one of `BITBUCKET_EMAIL` and `BITBUCKET_API_TOKEN` is set
- **THEN** requests are anonymous
- **AND** a warning names the variable that was set and the one that is missing

#### Scenario: Anonymous by default

- **WHEN** no credential variable is set
- **THEN** requests carry no `Authorization` header

### Requirement: Credentials are never disclosed

No credential value SHALL appear in any output, report, error message, log entry
or state file. Wherever the credential in use is described, it SHALL be shown by
its kind with the secret replaced by a marker.

#### Scenario: A failed request does not echo the credential

- **WHEN** a request with a bearer token is rejected
- **THEN** the error message does not contain the token

#### Scenario: Debug logging redacts

- **WHEN** retrieval runs with debug logging enabled
- **THEN** each logged request URL is accompanied by the credential kind and a redaction marker, and never by the secret

### Requirement: Retrieval is read-only and complete

Retrieval SHALL issue only HTTP GET requests. It SHALL follow the comments
collection's `next` cursors until a page carries none, and return every page's
comments in the order received. A cursor repeating an already requested URL SHALL
stop retrieval with an error. The comments request SHALL ask for resolution
detail, so a resolved thread reports by whom and when.

#### Scenario: Multiple pages are concatenated

- **WHEN** the comments collection spans three pages
- **THEN** the comments from all three pages are returned in page order

#### Scenario: A repeating cursor terminates

- **WHEN** a page's `next` cursor names a URL already requested
- **THEN** retrieval stops with an error naming the repeated URL

#### Scenario: Only GET is issued

- **WHEN** any retrieval runs
- **THEN** every request issued uses the GET method

### Requirement: Retrieval failures name their remedy

The system SHALL distinguish these retrieval failures, each with its own message:
- a transport failure
- a rejected credential
- an API token carrying no Bitbucket scopes, told apart from a rejected credential by the response body
- a pull request that does not exist or is not visible
- an unexpected status, with that status
- a response that does not decode

A failed retrieval SHALL leave the review state unchanged.

#### Scenario: A token without Bitbucket scopes

- **WHEN** the API answers 401 with a body stating the token has no Bitbucket scopes
- **THEN** the error states that the token needs the `read:pullrequest:bitbucket` scope

#### Scenario: Not found or not visible

- **WHEN** the API answers 404
- **THEN** the error states that the pull request does not exist or is not visible to the credential

#### Scenario: State is untouched on failure

- **WHEN** retrieval fails for any reason
- **THEN** the review state file is unchanged

### Requirement: Retrieved feedback is normalised onto the import document

The system SHALL normalise a retrieved pull request and its comments onto the
import document that `import <file>` accepts. Every retrieved comment SHALL appear
exactly once. Replies SHALL nest under their parent, ordered by creation time and
then by identifier. Identical input SHALL yield an identical document. Recorded
payloads SHALL normalise to the document upstream `bb-feedback` produced for them.

#### Scenario: Matches upstream output

- **WHEN** a recorded pull request and comments payload is normalised
- **THEN** the encoded document equals the one upstream `bb-feedback` produced from the same payloads

#### Scenario: Reply chains are rebuilt

- **WHEN** comments form a reply chain four levels deep
- **THEN** the document nests each reply under its parent at the same depth

### Requirement: Anchors map onto the four position cases

A comment's pre-image and post-image line fields SHALL map onto positions:
- neither present is the whole file
- pre-image only is original lines
- post-image only is updated lines
- both present is unchanged, carrying both spans

A range start SHALL make the position a span ending at the line. An anchor with no
path SHALL make the thread review-level.

#### Scenario: Each presence combination maps to its case

- **WHEN** each combination of present and absent pre-image and post-image fields is normalised
- **THEN** each yields its stated position case

#### Scenario: An inverted range is clamped

- **WHEN** a range start lies beyond its end line
- **THEN** the position is the single end line and a warning finding is recorded

#### Scenario: A line below 1 is the whole file

- **WHEN** an anchor names line 0 or a negative line
- **THEN** the position is the whole file on the anchor's path and a warning finding is recorded

### Requirement: Authors, bodies and resolution keep their distinctions

An author's source discriminator SHALL map to human, application with its kind,
or unknown with the reported value kept. It SHALL NOT default to human. A comment
marked deleted, or carrying no text, SHALL have a deleted body. A thread SHALL be
unresolved, resolved with by whom and when, or resolved without detail when the
source withheld it.

#### Scenario: An author with no name

- **WHEN** an author's display name is empty or blank
- **THEN** the author is named `unknown` and a warning finding is recorded

#### Scenario: An unrecognised author type

- **WHEN** an author's type is neither a person nor an application
- **THEN** the author is of unknown kind carrying the reported type, and a warning finding is recorded

#### Scenario: Resolution detail withheld

- **WHEN** a thread's resolution arrives as an empty object
- **THEN** the thread is resolved without detail and a warning finding is recorded

### Requirement: Normalisation anomalies become findings

Normalisation SHALL complete for any decodable payload. Each structural anomaly
SHALL become a finding with a severity (debug, warning or error), a stable code
and a detail naming the affected entity. Anomalies include orphan replies, reply
cycles, duplicate identifiers, inverted ranges, unparseable timestamps and
unknown states.

#### Scenario: An orphan reply is promoted

- **WHEN** a reply names a parent absent from the retrieved set
- **THEN** the reply becomes a thread root and a warning finding names both identifiers

#### Scenario: A reply cycle is broken

- **WHEN** comments name each other as parents
- **THEN** the lowest identifier becomes a thread root, every comment appears once, and an error finding is recorded

#### Scenario: A duplicate identifier

- **WHEN** two retrieved comments share an identifier
- **THEN** the later one is dropped and an error finding is recorded

### Requirement: The normalised document is validated independently of its provider

The system SHALL check a normalised document for invariants that need nothing but
the document:
- unique comment identifiers
- threads in the collection their location names
- non-empty anchor paths
- replies not older than their parents
- resolved threads carrying detail
- reconciliation of the comments present, less those without a body, against the
  reported comment count

These checks SHALL NOT depend on the provider.

#### Scenario: Count reconciliation detects a lost page

- **WHEN** the comments present less those without a body differ from the reported count
- **THEN** a warning finding states the three numbers

#### Scenario: Deleted comments are noted

- **WHEN** the document holds comments without a body
- **THEN** a debug finding states how many of the total have no body

### Requirement: Findings are reported with the import

A fetch SHALL report the same counts and paths as a file import. It SHALL also
report every finding at warning or error severity, most severe first, each with
its severity, code and detail. Debug findings SHALL be logged at DEBUG only. An
error finding SHALL NOT prevent the merge.

#### Scenario: Warnings appear in the report

- **WHEN** a fetch records a warning finding
- **THEN** the report includes a line with `warning`, the finding's code and its detail

#### Scenario: Debug findings stay out of the report

- **WHEN** a fetch records only debug findings
- **THEN** the report contains no finding lines

### Requirement: A source branch mismatch is reported

When the fetched pull request's source branch differs from the review's source
branch, the report SHALL name both branches. The merge SHALL still proceed. A URL
can deliberately name a pull request for another branch.

#### Scenario: Fetching a pull request for another branch

- **WHEN** a URL names a pull request whose source branch is `other` while the review's is `feature-x`
- **THEN** the report names `other` and `feature-x`
- **AND** the feedback is merged

### Requirement: Fetched feedback merges as an imported source

Fetched feedback SHALL be validated against the import document schema, then
mapped, placed, merged and persisted by the same rules as a file import. A pull
request SHALL have the same source identity whether its feedback was fetched or
imported from a file. Fetching again therefore replaces that source's feedback,
and keeps local replies and local status.

#### Scenario: Normalised output conforms to the schema

- **WHEN** a recorded pull request and comments payload is normalised and encoded
- **THEN** the document is valid against the import document schema

#### Scenario: Fetch after a file import of the same pull request

- **WHEN** a pull request's document was imported from a file and the same pull request is then fetched
- **THEN** the review holds one copy of each of its comments under one source

### Requirement: The GUI fetches feedback

The GUI SHALL offer a control that fetches feedback for the current branch's pull
request. It SHALL also accept a pasted pull request URL. Both SHALL run the same
retrieval, normalisation and merge as the CLI. The outcome SHALL appear in the
import report banner. A failure SHALL show its message and leave the review
unchanged. The control SHALL be unavailable while a fetch is in progress.

#### Scenario: Fetch for the current branch

- **WHEN** the reviewer fetches with no URL and one open pull request matches the branch
- **THEN** its feedback appears in the review without a manual refresh
- **AND** the banner shows the report

#### Scenario: Fetch by URL

- **WHEN** the reviewer pastes a pull request URL and fetches
- **THEN** that pull request's feedback is merged and the banner shows the report

#### Scenario: Fetch fails

- **WHEN** retrieval fails
- **THEN** the banner shows the failure's message and the review is unchanged
