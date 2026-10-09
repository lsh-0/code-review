package bitbucket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// sends one request. `*http.Client` satisfies it in production. A test supplies one
// that answers from recorded files, so the retrieval code under test is the code
// that runs.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// the largest response body read. The comments collection is read at 100 comments
// per page, and the largest recorded page is about 200 KB.
const maxBody = 32 << 20

// why a request did not produce a usable response.
//
// `NoBitbucketScopes` is separate from `Unauthorized` because the remedies differ,
// and it is the most likely first-run failure: a token created from the general
// Atlassian account page carries no Bitbucket scopes.
type ErrorKind int

const (
	Transport ErrorKind = iota
	Unauthorized
	NoBitbucketScopes
	NotFound
	UnexpectedStatus
	CursorLoop
	ForeignCursor
	Undecodable
)

// a retrieval failure. `Detail` holds a response body, a URL or a decoding error,
// and never a request header, so no error can carry a credential.
type Error struct {
	Kind   ErrorKind
	Status int
	Detail string
}

// the failure with its remedy where there is one.
func (e *Error) Error() string {
	switch e.Kind {
	case Transport:
		return "could not reach Bitbucket: " + e.Detail
	case Unauthorized:
		return "Bitbucket rejected the credential. Check that it has not expired, and that an access token " +
			"is set as " + AccessTokenVar + " while an API token is set as " + APITokenVar + " with " + EmailVar + "."
	case NoBitbucketScopes:
		return "the API token carries no Bitbucket scopes. Create one through the scoped token flow with " +
			"read:pullrequest:bitbucket selected; a token created from the general Atlassian account page " +
			"has Jira and Confluence scopes only and authenticates nowhere in Bitbucket."
	case NotFound:
		return "no such pull request, or it is not visible to this credential. Bitbucket answers 404 for " +
			"both, so these cannot be told apart. A private repository needs " + AccessTokenVar + ", or " +
			EmailVar + " with " + APITokenVar + "."
	case UnexpectedStatus:
		return fmt.Sprintf("unexpected HTTP %d: %s", e.Status, strings.TrimSpace(e.Detail))
	case CursorLoop:
		return "pagination did not advance: " + e.Detail + " repeated"
	case ForeignCursor:
		return "pagination cursor leaves the Bitbucket API and was not followed: " + e.Detail
	case Undecodable:
		return "could not decode the response: " + e.Detail
	default:
		return e.Detail
	}
}

// reports whether a body names the missing-scope failure. Bitbucket answers 401 for
// both a rejected credential and a token without Bitbucket scopes, and the body text
// is the only signal.
func mentionsMissingScopes(body string) bool {
	return strings.Contains(body, "no Bitbucket scopes")
}

// maps a status and body onto success or a specific error.
func classify(status int, body string) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusUnauthorized && mentionsMissingScopes(body):
		return &Error{Kind: NoBitbucketScopes, Status: status, Detail: body}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &Error{Kind: Unauthorized, Status: status, Detail: body}
	case status == http.StatusNotFound:
		return &Error{Kind: NotFound, Status: status, Detail: body}
	default:
		return &Error{Kind: UnexpectedStatus, Status: status, Detail: body}
	}
}

// the client retrieval runs through: where to send, and as whom. `Log`, when set,
// receives each request URL with the credential redacted.
type Client struct {
	Doer        Doer
	Credentials Credentials
	Log         func(url string, credentials string)
}

// issues an authenticated GET for `url` and returns the body of a successful
// response.
func (c Client) get(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &Error{Kind: Transport, Detail: err.Error()}
	}
	request.Header.Set("Accept", "application/json")
	c.Credentials.authorize(request)
	if c.Log != nil {
		c.Log(url, c.Credentials.Redacted())
	}

	response, err := c.Doer.Do(request)
	if err != nil {
		return nil, &Error{Kind: Transport, Detail: transportDetail(err)}
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody))
	if err != nil {
		return nil, &Error{Kind: Transport, Detail: transportDetail(err)}
	}
	if err := classify(response.StatusCode, string(body)); err != nil {
		return nil, err
	}
	return body, nil
}

// the cause of a transport failure. `*url.Error` repeats the request URL, which
// carries no credential, so it is kept.
func transportDetail(err error) string {
	if errors.Is(err, context.Canceled) {
		return "the request was cancelled"
	}
	return err.Error()
}
