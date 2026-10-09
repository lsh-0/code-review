package bitbucket

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// a `Doer` answering from recorded bodies keyed by URL. Every request is recorded,
// so a test can assert how many were issued, in what order, and with which method.
// An unknown URL answers 404.
type replay struct {
	routes   map[string]reply
	requests []*http.Request
}

type reply struct {
	status int
	body   string
}

func newReplay(routes map[string]string) *replay {
	replies := make(map[string]reply, len(routes))
	for url, body := range routes {
		replies[url] = reply{status: http.StatusOK, body: body}
	}
	return &replay{routes: replies}
}

func (r *replay) Do(request *http.Request) (*http.Response, error) {
	r.requests = append(r.requests, request)
	answer, found := r.routes[request.URL.String()]
	if !found {
		answer = reply{status: http.StatusNotFound, body: `{"type":"error"}`}
	}
	return &http.Response{
		StatusCode: answer.status,
		Body:       io.NopCloser(strings.NewReader(answer.body)),
		Header:     http.Header{},
	}, nil
}

func (r *replay) urls() []string {
	urls := make([]string, 0, len(r.requests))
	for _, request := range r.requests {
		urls = append(urls, request.URL.String())
	}
	return urls
}

// the recorded pages carry real `next` cursors captured from the live API, used
// verbatim as route keys.
func pagedRoutes(t *testing.T) map[string]string {
	t.Helper()
	first := fixturePage(t, "aui_5405_comments_page1.json")
	second := fixturePage(t, "aui_5405_comments_page2.json")
	return map[string]string{
		commentsURL(aui):    string(readFixture(t, "aui_5405_comments_page1.json")),
		*first.Next:         string(readFixture(t, "aui_5405_comments_page2.json")),
		*second.Next:        string(readFixture(t, "aui_5405_comments_page3.json")),
		pullRequestURL(aui): string(readFixture(t, "aui_5405_pr.json")),
	}
}

func TestCommentsAcrossThreePages(t *testing.T) {
	doer := newReplay(pagedRoutes(t))

	actual, err := Client{Doer: doer}.Comments(context.Background(), aui)

	if err != nil {
		t.Fatal(err)
	}
	if len(actual) != 5 {
		t.Errorf("comments: got %d, want 5", len(actual))
	}
	if len(doer.requests) != 3 {
		t.Errorf("requests: got %d, want 3", len(doer.requests))
	}
	unique := map[int]bool{}
	for _, comment := range actual {
		unique[comment.ID] = true
	}
	if len(unique) != 5 {
		t.Errorf("distinct comments: got %d, want 5", len(unique))
	}
}

func TestCommentsSinglePage(t *testing.T) {
	doer := newReplay(map[string]string{commentsURL(aui): string(readFixture(t, "synthetic_3754_comments.json"))})

	actual, err := Client{Doer: doer}.Comments(context.Background(), aui)

	if err != nil {
		t.Fatal(err)
	}
	if len(actual) != 25 || len(doer.requests) != 1 {
		t.Errorf("got %d comments in %d requests, want 25 in 1", len(actual), len(doer.requests))
	}
}

func TestCommentsEmptyCollection(t *testing.T) {
	doer := newReplay(map[string]string{commentsURL(aui): `{"values": [], "size": 0}`})

	actual, err := Client{Doer: doer}.Comments(context.Background(), aui)

	if err != nil || len(actual) != 0 {
		t.Errorf("got %v, %v; want no comments and no error", actual, err)
	}
}

// the guard is checked before the request, so the repeated URL is never issued a
// second time.
func TestRepeatedCursorTerminates(t *testing.T) {
	selfReferential := `{"values": [], "next": "` + commentsURL(aui) + `"}`
	doer := newReplay(map[string]string{commentsURL(aui): selfReferential})

	_, err := Client{Doer: doer}.Comments(context.Background(), aui)

	var actual *Error
	if !errors.As(err, &actual) || actual.Kind != CursorLoop {
		t.Fatalf("got %v, want a cursor loop", err)
	}
	if len(doer.requests) != 1 {
		t.Errorf("requests: got %d, want 1", len(doer.requests))
	}
	if !strings.Contains(err.Error(), commentsURL(aui)) {
		t.Errorf("error %q does not name the repeated URL", err)
	}
}

// a cursor leaving the API would send the credential to another host. It is
// refused without a request.
func TestForeignCursorIsNotFollowed(t *testing.T) {
	given := []string{
		"https://evil.example/2.0/repositories/x",
		"http://api.bitbucket.org/2.0/repositories/x",
		"https://api.bitbucket.org.evil.example/2.0",
	}
	for _, cursor := range given {
		doer := newReplay(map[string]string{commentsURL(aui): `{"values": [], "next": "` + cursor + `"}`})
		credentials, _ := ResolveCredentials(env(map[string]string{AccessTokenVar: "secret"}))

		_, err := Client{Doer: doer, Credentials: credentials}.Comments(context.Background(), aui)

		var actual *Error
		if !errors.As(err, &actual) || actual.Kind != ForeignCursor {
			t.Errorf("%s: got %v, want a foreign cursor error", cursor, err)
		}
		if len(doer.requests) != 1 {
			t.Errorf("%s: requests: got %d, want 1", cursor, len(doer.requests))
		}
	}
}

func TestPullRequestRetrieval(t *testing.T) {
	doer := newReplay(map[string]string{pullRequestURL(aui): string(readFixture(t, "synthetic_3754_pr.json"))})

	actual, err := Client{Doer: doer}.PullRequest(context.Background(), aui)

	if err != nil || actual.ID != 3754 {
		t.Errorf("got %d, %v; want 3754", actual.ID, err)
	}
}

func TestUndecodableResponse(t *testing.T) {
	doer := newReplay(map[string]string{pullRequestURL(aui): `{"id": "x"}`})

	_, err := Client{Doer: doer}.PullRequest(context.Background(), aui)

	var actual *Error
	if !errors.As(err, &actual) || actual.Kind != Undecodable {
		t.Errorf("got %v, want an undecodable error", err)
	}
}

// every request retrieval issues is a GET, carries `Accept`, and carries the
// credential's header.
func TestEveryRequestIsAnAuthenticatedGet(t *testing.T) {
	doer := newReplay(pagedRoutes(t))
	credentials, _ := ResolveCredentials(env(map[string]string{AccessTokenVar: "tok"}))
	client := Client{Doer: doer, Credentials: credentials}

	if _, err := client.PullRequest(context.Background(), aui); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Comments(context.Background(), aui); err != nil {
		t.Fatal(err)
	}

	for _, request := range doer.requests {
		if request.Method != http.MethodGet {
			t.Errorf("%s: method %s, want GET", request.URL, request.Method)
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Errorf("%s: missing Accept", request.URL)
		}
		if request.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("%s: missing the credential", request.URL)
		}
	}
}

func TestLogReceivesRedactedCredentials(t *testing.T) {
	doer := newReplay(map[string]string{pullRequestURL(aui): string(readFixture(t, "aui_5405_pr.json"))})
	credentials, _ := ResolveCredentials(env(map[string]string{AccessTokenVar: "ATATTsecret"}))
	logged := make([]string, 0)
	client := Client{Doer: doer, Credentials: credentials, Log: func(url string, as string) {
		logged = append(logged, url+" "+as)
	}}

	if _, err := client.PullRequest(context.Background(), aui); err != nil {
		t.Fatal(err)
	}

	expected := pullRequestURL(aui) + " access token (<redacted>)"
	if len(logged) != 1 || logged[0] != expected {
		t.Errorf("got %q, want [%q]", logged, expected)
	}
}

func TestClassify(t *testing.T) {
	given := []struct {
		name     string
		status   int
		body     string
		expected ErrorKind
		ok       bool
	}{
		{"200 succeeds", 200, "{}", 0, true},
		{"204 succeeds", 204, "", 0, true},
		// the exact body Bitbucket returns for a token created without Bitbucket scopes.
		{"401 naming missing scopes", 401, `{"type": "error", "error": {"message": "API Token provided has no Bitbucket scopes."}}`, NoBitbucketScopes, false},
		{"401 for a rejected token", 401, `{"type": "error", "error": {"message": "Token is invalid, expired, or not supported for this endpoint."}}`, Unauthorized, false},
		{"403 is unauthorized", 403, "{}", Unauthorized, false},
		{"404 is not found", 404, "{}", NotFound, false},
		{"500 is unexpected", 500, "boom", UnexpectedStatus, false},
	}
	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			err := classify(test.status, test.body)
			if test.ok {
				if err != nil {
					t.Errorf("got %v, want success", err)
				}
				return
			}
			var actual *Error
			if !errors.As(err, &actual) || actual.Kind != test.expected {
				t.Errorf("got %v, want kind %v", err, test.expected)
			}
		})
	}
}

func TestErrorMessagesNameTheirRemedy(t *testing.T) {
	given := []struct {
		kind   ErrorKind
		needle string
	}{
		{NoBitbucketScopes, "read:pullrequest:bitbucket"},
		{NotFound, "not visible"},
		{NotFound, AccessTokenVar},
		{Unauthorized, AccessTokenVar},
		{UnexpectedStatus, "unexpected HTTP 500"},
	}
	for _, test := range given {
		actual := (&Error{Kind: test.kind, Status: 500, Detail: "x"}).Error()
		if !strings.Contains(actual, test.needle) {
			t.Errorf("kind %v: %q does not contain %q", test.kind, actual, test.needle)
		}
	}
}

// a rejected request's error never carries the token it was sent with.
func TestARejectedRequestDoesNotEchoTheCredential(t *testing.T) {
	doer := &replay{routes: map[string]reply{
		pullRequestURL(aui): {status: 401, body: `{"type":"error","error":{"message":"Token is invalid"}}`},
	}}
	credentials, _ := ResolveCredentials(env(map[string]string{AccessTokenVar: "ATATTsecret"}))

	_, err := Client{Doer: doer, Credentials: credentials}.PullRequest(context.Background(), aui)

	if err == nil || strings.Contains(err.Error(), "ATATTsecret") {
		t.Errorf("got %v, want an error without the token", err)
	}
}

func TestTransportFailureIsReported(t *testing.T) {
	failing := doerFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("connection refused") })

	_, err := Client{Doer: failing}.PullRequest(context.Background(), aui)

	var actual *Error
	if !errors.As(err, &actual) || actual.Kind != Transport || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("got %v, want a transport error naming the cause", err)
	}
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

// a page of open pull requests with the given numbers.
func pullRequestListPage(numbers ...int) string {
	values := make([]string, 0, len(numbers))
	for _, number := range numbers {
		values = append(values, `{"id": `+strconv.Itoa(number)+`, "author": {"display_name": "A"}, "created_on": "t",
			"source": {"branch": {"name": "feature-x"}}, "destination": {"branch": {"name": "master"}}}`)
	}
	return `{"values": [` + strings.Join(values, ",") + `]}`
}

func TestFindOpenForBranch(t *testing.T) {
	repo := aui.Repository

	t.Run("one match", func(t *testing.T) {
		doer := newReplay(map[string]string{openPullRequestsURL(repo, "feature-x"): pullRequestListPage(42)})

		actual, err := Client{Doer: doer}.FindOpenForBranch(context.Background(), repo, "feature-x")

		expected := PullRequestRef{Repository: repo, Number: 42}
		if err != nil || actual != expected {
			t.Errorf("got %+v, %v; want %+v", actual, err, expected)
		}
	})

	t.Run("no match", func(t *testing.T) {
		doer := newReplay(map[string]string{openPullRequestsURL(repo, "feature-x"): pullRequestListPage()})

		_, err := Client{Doer: doer}.FindOpenForBranch(context.Background(), repo, "feature-x")

		var actual *NoMatchError
		if !errors.As(err, &actual) {
			t.Fatalf("got %v, want no match", err)
		}
		if !strings.Contains(err.Error(), `"feature-x"`) || !strings.Contains(err.Error(), "atlassian/aui") {
			t.Errorf("error %q does not name the branch and the repository", err)
		}
	})

	t.Run("two matches", func(t *testing.T) {
		doer := newReplay(map[string]string{openPullRequestsURL(repo, "feature-x"): pullRequestListPage(42, 43)})

		_, err := Client{Doer: doer}.FindOpenForBranch(context.Background(), repo, "feature-x")

		var actual *ManyMatchesError
		if !errors.As(err, &actual) || len(actual.Matches) != 2 {
			t.Fatalf("got %v, want two matches", err)
		}
		for _, needle := range []string{"#42", "#43", "pull-requests/43", "give a URL"} {
			if !strings.Contains(err.Error(), needle) {
				t.Errorf("error %q does not contain %q", err, needle)
			}
		}
	})

	t.Run("across pages", func(t *testing.T) {
		second := "https://api.bitbucket.org/2.0/repositories/atlassian/aui/pullrequests?page=2"
		firstPage := strings.TrimSuffix(pullRequestListPage(), "}") + `, "next": "` + second + `"}`
		doer := newReplay(map[string]string{
			openPullRequestsURL(repo, "feature-x"): firstPage,
			second:                                 pullRequestListPage(44),
		})

		actual, err := Client{Doer: doer}.FindOpenForBranch(context.Background(), repo, "feature-x")

		if err != nil || actual.Number != 44 {
			t.Errorf("got %+v, %v; want #44", actual, err)
		}
	})
}

func TestCancelledContextStopsRetrieval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelling := doerFunc(func(request *http.Request) (*http.Response, error) { return nil, request.Context().Err() })

	_, err := Client{Doer: cancelling}.PullRequest(ctx, aui)

	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("got %v, want a cancellation", err)
	}
}
