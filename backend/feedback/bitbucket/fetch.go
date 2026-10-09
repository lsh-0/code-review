package bitbucket

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// retrieves the pull request's metadata.
func (c Client) PullRequest(ctx context.Context, ref PullRequestRef) (wirePullRequest, error) {
	body, err := c.get(ctx, pullRequestURL(ref))
	if err != nil {
		return wirePullRequest{}, err
	}
	pr, err := decodePullRequest(body)
	if err != nil {
		return wirePullRequest{}, &Error{Kind: Undecodable, Detail: err.Error()}
	}
	return pr, nil
}

// retrieves every comment on the pull request, following `next` cursors until a
// page carries none, in the order received.
func (c Client) Comments(ctx context.Context, ref PullRequestRef) ([]wireComment, error) {
	return collect(ctx, c, commentsURL(ref), decodeComment)
}

// the open pull requests whose source branch is `branch`. Exactly one match is
// returned. None, or several, is an error: guessing among several would merge
// feedback from the wrong pull request into a review.
func (c Client) FindOpenForBranch(ctx context.Context, repo Repository, branch string) (PullRequestRef, error) {
	matches, err := collect(ctx, c, openPullRequestsURL(repo, branch), decodeListedPullRequest)
	if err != nil {
		return PullRequestRef{}, err
	}
	switch len(matches) {
	case 0:
		return PullRequestRef{}, &NoMatchError{Repository: repo, Branch: branch}
	case 1:
		return PullRequestRef{Repository: repo, Number: matches[0].ID}, nil
	default:
		refs := make([]PullRequestRef, 0, len(matches))
		for _, match := range matches {
			refs = append(refs, PullRequestRef{Repository: repo, Number: match.ID})
		}
		return PullRequestRef{}, &ManyMatchesError{Branch: branch, Matches: refs}
	}
}

// decodes one element of a pull request listing. The listing's elements carry the
// same fields as a single pull request.
func decodeListedPullRequest(data []byte, _ string) (wirePullRequest, error) {
	return decodePullRequest(data)
}

// no open pull request has the branch as its source.
type NoMatchError struct {
	Repository Repository
	Branch     string
}

func (e *NoMatchError) Error() string {
	return fmt.Sprintf("no open pull request in %s/%s has source branch %q",
		e.Repository.Workspace, e.Repository.Slug, e.Branch)
}

// several open pull requests have the branch as their source.
type ManyMatchesError struct {
	Branch  string
	Matches []PullRequestRef
}

func (e *ManyMatchesError) Error() string {
	lines := make([]string, 0, len(e.Matches))
	for _, match := range e.Matches {
		lines = append(lines, fmt.Sprintf("  #%d %s", match.Number, match.WebURL()))
	}
	return fmt.Sprintf("%d open pull requests have source branch %q; give a URL to select one:\n%s",
		len(e.Matches), e.Branch, strings.Join(lines, "\n"))
}

// reports whether a cursor stays on the API: the same scheme and host. A cursor
// naming another host would send the credential there.
func onAPI(cursor string) bool {
	parsed, err := url.Parse(cursor)
	return err == nil && parsed.Scheme == apiScheme && parsed.Host == apiHost
}

// every value of a cursor-paginated collection starting at `first`.
//
// Terminates on any input. A cursor repeating a URL already requested is an error
// before it is requested again, and a cursor leaving the API is refused.
func collect[T any](ctx context.Context, c Client, first string, decode func([]byte, string) (T, error)) ([]T, error) {
	values := make([]T, 0)
	seen := make(map[string]bool)
	next := first
	for {
		if seen[next] {
			return nil, &Error{Kind: CursorLoop, Detail: next}
		}
		if !onAPI(next) {
			return nil, &Error{Kind: ForeignCursor, Detail: next}
		}
		seen[next] = true

		body, err := c.get(ctx, next)
		if err != nil {
			return nil, err
		}
		page, err := decodePage(body, decode)
		if err != nil {
			return nil, &Error{Kind: Undecodable, Detail: err.Error()}
		}
		values = append(values, page.Values...)
		if page.Next == nil {
			return values, nil
		}
		next = *page.Next
	}
}
