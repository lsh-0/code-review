// Package bitbucket retrieves review feedback from a Bitbucket Cloud pull request
// and normalises it onto the provider-independent `feedback.Document`.
//
// Everything Bitbucket-specific stays in this package: request URLs, credentials,
// the API's wire format and its ambiguities, and the errors the API returns. Nothing
// outside it needs to know which provider a document came from.
//
// Retrieval is GET only, against `api.bitbucket.org` only, through a `Doer` the
// caller supplies. Nothing here reads the environment or opens a socket by itself.
package bitbucket

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// the API every request goes to. A pagination cursor naming any other scheme or host
// is refused, so a response cannot redirect a credential elsewhere.
const (
	apiScheme = "https"
	apiHost   = "api.bitbucket.org"
	apiBase   = apiScheme + "://" + apiHost + "/2.0"
)

// the host of Bitbucket Cloud's web interface and git remotes.
const webHost = "bitbucket.org"

// identifies a repository by its workspace and slug.
type Repository struct {
	Workspace string
	Slug      string
}

// identifies one pull request.
type PullRequestRef struct {
	Repository
	Number int
}

// the web URL of the pull request, as a reviewer would open it.
func (r PullRequestRef) WebURL() string {
	return fmt.Sprintf("https://%s/%s/%s/pull-requests/%d", webHost, r.Workspace, r.Slug, r.Number)
}

// percent-encodes everything outside the unreserved set of RFC 3986, so a slug is
// one path segment whatever it contains.
func percentEncode(segment string) string {
	var builder strings.Builder
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		if isUnreserved(c) {
			builder.WriteByte(c)
			continue
		}
		fmt.Fprintf(&builder, "%%%02X", c)
	}
	return builder.String()
}

func isUnreserved(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

func repositoryURL(repo Repository) string {
	return fmt.Sprintf("%s/repositories/%s/%s", apiBase, percentEncode(repo.Workspace), percentEncode(repo.Slug))
}

// `/2.0/repositories/{workspace}/{slug}/pullrequests/{number}`.
func pullRequestURL(ref PullRequestRef) string {
	return fmt.Sprintf("%s/pullrequests/%d", repositoryURL(ref.Repository), ref.Number)
}

// the comments of a pull request, at the maximum page size, with resolution detail.
//
// The field expansion is required. Without it Bitbucket returns `"resolution": {}`
// for every resolved thread: resolution survives but by whom and when are elided,
// and nothing in the response says so. `+` makes the expansion additive, so every
// default field is still returned.
func commentsURL(ref PullRequestRef) string {
	return pullRequestURL(ref) + "/comments?pagelen=100&fields=%2Bvalues.resolution.*"
}

// the open pull requests whose source branch is `branch`.
//
// The branch is quoted in Bitbucket's query language, with `"` and `\` escaped, and
// the whole query is percent-encoded, so a branch name cannot change the query's
// meaning.
func openPullRequestsURL(repo Repository, branch string) string {
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(branch)
	query := `source.branch.name="` + quoted + `"`
	return repositoryURL(repo) + "/pullrequests?state=OPEN&pagelen=50&q=" + url.QueryEscape(query)
}

// reads a pull request from its web URL:
// `https://bitbucket.org/<workspace>/<slug>/pull-requests/<number>`. Trailing path
// segments such as `/overview` or `/diff`, a query and a fragment are accepted.
func ParsePullRequestURL(raw string) (PullRequestRef, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return PullRequestRef{}, fmt.Errorf("not a pull request URL: %q", raw)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return PullRequestRef{}, fmt.Errorf("not a pull request URL: %q", raw)
	}
	host := strings.ToLower(parsed.Hostname())
	if host != webHost && host != "www."+webHost {
		return PullRequestRef{}, fmt.Errorf("unsupported pull request host %q in %q: only %s is supported", host, raw, webHost)
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 4 || segments[0] == "" || segments[1] == "" || segments[2] != "pull-requests" {
		return PullRequestRef{}, fmt.Errorf("URL does not name a pull request: %q", raw)
	}
	number, err := strconv.Atoi(segments[3])
	if err != nil || number < 1 {
		return PullRequestRef{}, fmt.Errorf("URL does not name a pull request number: %q", raw)
	}
	return PullRequestRef{
		Repository: Repository{Workspace: segments[0], Slug: segments[1]},
		Number:     number,
	}, nil
}

// reads a repository from a git remote URL. Accepts the scp-like SSH form
// `git@bitbucket.org:<workspace>/<slug>.git`, `ssh://git@bitbucket.org/...`, and
// `https://[user@]bitbucket.org/...`, each with or without `.git`.
//
// Returns an error naming the host for a remote on any other host.
func ParseRemote(raw string) (Repository, error) {
	remote := strings.TrimSpace(raw)
	host, path, ok := splitRemote(remote)
	if !ok {
		return Repository{}, fmt.Errorf("not a recognised git remote URL: %q", raw)
	}
	if strings.ToLower(host) != webHost {
		return Repository{}, fmt.Errorf("remote %q is on %q: only %s is supported", raw, host, webHost)
	}
	segments := strings.Split(strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/")
	if len(segments) != 2 || segments[0] == "" || segments[1] == "" {
		return Repository{}, fmt.Errorf("remote %q does not name a workspace and repository", raw)
	}
	return Repository{Workspace: segments[0], Slug: segments[1]}, nil
}

// the host and path of a remote URL, in either URL form or scp-like form.
func splitRemote(remote string) (string, string, bool) {
	if strings.Contains(remote, "://") {
		parsed, err := url.Parse(remote)
		if err != nil || parsed.Hostname() == "" {
			return "", "", false
		}
		switch parsed.Scheme {
		case "ssh", "https", "http", "git+ssh":
		default:
			return "", "", false
		}
		return parsed.Hostname(), parsed.Path, true
	}
	// scp-like: [user@]host:path. A colon before any slash separates host from path.
	before0, after, ok := strings.Cut(remote, ":")
	if !ok || strings.Contains(before0, "/") {
		return "", "", false
	}
	host := before0
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if host == "" {
		return "", "", false
	}
	return host, after, true
}
