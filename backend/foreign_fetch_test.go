package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"code-review/feedback"
	"code-review/model"
)

// the recorded aui 5405 pull request, as the API serves it.
const (
	auiAPI      = "https://api.bitbucket.org/2.0/repositories/atlassian/aui"
	auiPRURL    = auiAPI + "/pullrequests/5405"
	auiComments = auiPRURL + "/comments?pagelen=100&fields=%2Bvalues.resolution.*"
	auiWebURL   = "https://bitbucket.org/atlassian/aui/pull-requests/5405"
	// the open pull requests on branch `feature`, with the query as the client encodes it.
	auiOpenOnFeature = auiAPI + "/pullrequests?state=OPEN&pagelen=50&q=source.branch.name%3D%22feature%22"
)

// a `Doer` answering from recorded bodies keyed by URL, recording every request.
// An unknown URL answers 404.
type replayDoer struct {
	routes   map[string]replayReply
	requests []*http.Request
}

type replayReply struct {
	status int
	body   string
}

func (d *replayDoer) Do(request *http.Request) (*http.Response, error) {
	d.requests = append(d.requests, request)
	reply, found := d.routes[request.URL.String()]
	if !found {
		reply = replayReply{status: http.StatusNotFound, body: `{"type":"error"}`}
	}
	return &http.Response{StatusCode: reply.status, Body: io.NopCloser(strings.NewReader(reply.body)), Header: http.Header{}}, nil
}

func readBitbucketFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "bitbucket", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// routes serving the recorded aui 5405 pull request and its comments, and an open
// pull request listing on `feature` holding `open` pull request numbers.
func auiRoutes(t *testing.T, open ...int) map[string]replayReply {
	t.Helper()
	listing := make([]string, 0, len(open))
	for _, number := range open {
		listing = append(listing, `{"id": `+strconv.Itoa(number)+`, "author": {"display_name": "A"}, "created_on": "2026-01-01T00:00:00Z",
			"source": {"branch": {"name": "feature"}}, "destination": {"branch": {"name": "main"}}}`)
	}
	return map[string]replayReply{
		auiPRURL:         {http.StatusOK, readBitbucketFixture(t, "aui_5405_pr.json")},
		auiComments:      {http.StatusOK, readBitbucketFixture(t, "aui_5405_comments.json")},
		auiOpenOnFeature: {http.StatusOK, `{"values": [` + strings.Join(listing, ",") + `]}`},
	}
}

// substitutes the fetch environment for the duration of the test.
func useFetchEnvironment(t *testing.T, doer *replayDoer, env map[string]string) {
	t.Helper()
	previous := currentFetchEnvironment
	currentFetchEnvironment = func() fetchEnvironment {
		return fetchEnvironment{doer: doer, getenv: func(name string) string { return env[name] }}
	}
	t.Cleanup(func() { currentFetchEnvironment = previous })
}

// a repository on `feature` with `origin` on Bitbucket and the file the aui 5405
// inline comment addresses, so the comment has a line to be placed at.
func initRepoForAui(t *testing.T) string {
	t.Helper()
	dir := initGitRepo(t)
	gitInDir(t, dir, "remote", "add", "origin", "git@bitbucket.org:atlassian/aui.git")
	path := filepath.Join(dir, "tests", "a11y-unit-tests")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, 20)
	for n := 1; n <= 20; n++ {
		lines = append(lines, `  "line`+strconv.Itoa(n)+`": true,`)
	}
	if err := os.WriteFile(filepath.Join(path, "package.json"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInDir(t, dir, "add", ".")
	gitInDir(t, dir, "commit", "-q", "-m", "add package.json")
	return dir
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestFetchByURLMergesAndReports(t *testing.T) {
	dir := initRepoForAui(t)
	doer := &replayDoer{routes: auiRoutes(t)}
	useFetchEnvironment(t, doer, nil)
	runCLIInDir(t, dir, "start")

	code, out, errOut := runCLIInDir(t, dir, "fetch", auiWebURL+"/overview")

	if code != 0 {
		t.Fatalf("fetch failed: %s", errOut)
	}
	for _, expected := range []string{
		"imported 3 comments from bb:5405 (1 placed, 0 untethered, 1 replies)",
		"skipped 2 deleted or pending comments",
		`the pull request's source branch is "issue/release/9.12.x/DCA11Y-3520-explicit-outline-color-focus" but the review is of "feature"`,
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("report does not contain %q:\n%s", expected, out)
		}
	}
	if strings.Contains(out, "deleted_comments_present") {
		t.Errorf("a debug finding reached the report:\n%s", out)
	}

	_, list, _ := runCLIInDir(t, dir, "list")
	if !strings.Contains(list, `"origin": "bb:5405"`) {
		t.Errorf("list does not show the fetched comments' origin:\n%s", list)
	}
}

// with no URL, the pull request is found from `origin` and the review's branch.
func TestFetchDiscoversThePullRequestForTheBranch(t *testing.T) {
	dir := initRepoForAui(t)
	doer := &replayDoer{routes: auiRoutes(t, 5405)}
	useFetchEnvironment(t, doer, nil)
	runCLIInDir(t, dir, "start")

	code, out, errOut := runCLIInDir(t, dir, "fetch")

	if code != 0 {
		t.Fatalf("fetch failed: %s", errOut)
	}
	if !strings.Contains(out, "from bb:5405") {
		t.Errorf("report: %s", out)
	}
	if doer.requests[0].URL.String() != auiOpenOnFeature {
		t.Errorf("first request: got %s, want the open pull request listing", doer.requests[0].URL)
	}
}

func TestFetchFailuresLeaveTheStateFileUnchanged(t *testing.T) {
	given := []struct {
		name    string
		routes  func(t *testing.T) map[string]replayReply
		args    []string
		message string
	}{
		{"not found", func(t *testing.T) map[string]replayReply { return map[string]replayReply{} },
			[]string{"fetch", auiWebURL}, "no such pull request, or it is not visible"},
		{"no open pull request", func(t *testing.T) map[string]replayReply { return auiRoutes(t) },
			[]string{"fetch"}, `has source branch "feature"`},
		{"two open pull requests", func(t *testing.T) map[string]replayReply { return auiRoutes(t, 5405, 5406) },
			[]string{"fetch"}, "give a URL to select one"},
		{"not a pull request URL", func(t *testing.T) map[string]replayReply { return auiRoutes(t) },
			[]string{"fetch", "https://github.com/a/b/pull/1"}, "unsupported pull request host"},
		{"undecodable", func(t *testing.T) map[string]replayReply {
			return map[string]replayReply{auiPRURL: {http.StatusOK, `{"id": "x"}`}}
		}, []string{"fetch", auiWebURL}, "could not decode the response"},
	}
	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			dir := initRepoForAui(t)
			useFetchEnvironment(t, &replayDoer{routes: test.routes(t)}, nil)
			runCLIInDir(t, dir, "start")
			statePath := resolvedStatePath(t, dir)
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}

			code, _, errOut := runCLIInDir(t, dir, test.args...)

			if code == 0 {
				t.Fatal("expected a non-zero exit")
			}
			if !strings.Contains(errOut, test.message) {
				t.Errorf("stderr does not contain %q:\n%s", test.message, errOut)
			}
			after, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Error("the state file changed on a failed fetch")
			}
		})
	}
}

func TestFetchWithoutABitbucketOriginSuggestsAURL(t *testing.T) {
	dir := initGitRepo(t)
	useFetchEnvironment(t, &replayDoer{routes: map[string]replayReply{}}, nil)
	runCLIInDir(t, dir, "start")

	code, _, errOut := runCLIInDir(t, dir, "fetch")

	if code == 0 || !strings.Contains(errOut, "give a pull request URL instead") {
		t.Errorf("got exit %d, stderr %q", code, errOut)
	}
}

func TestFetchRejectsTwoArguments(t *testing.T) {
	dir := initRepoForAui(t)
	useFetchEnvironment(t, &replayDoer{routes: map[string]replayReply{}}, nil)
	runCLIInDir(t, dir, "start")

	code, _, errOut := runCLIInDir(t, dir, "fetch", auiWebURL, "extra")

	if code == 0 || !strings.Contains(errOut, "usage: code-review fetch [<url>]") {
		t.Errorf("got exit %d, stderr %q", code, errOut)
	}
}

// a pull request has one source identity whichever way its feedback arrived, so
// fetching after importing the same pull request's document replaces it.
func TestFetchAfterImportOfTheSamePullRequestKeepsOneCopy(t *testing.T) {
	dir := initRepoForAui(t)
	useFetchEnvironment(t, &replayDoer{routes: auiRoutes(t)}, nil)
	upstream := readBitbucketFixture(t, "aui_5405.expected.json")
	runCLIInDir(t, dir, "start")
	document := filepath.Join(dir, "5405.json")
	if err := os.WriteFile(document, []byte(upstream), 0o644); err != nil {
		t.Fatal(err)
	}

	if code, _, errOut := runCLIInDir(t, dir, "import", document); code != 0 {
		t.Fatalf("import failed: %s", errOut)
	}
	if code, _, errOut := runCLIInDir(t, dir, "fetch", auiWebURL); code != 0 {
		t.Fatalf("fetch failed: %s", errOut)
	}

	review, err := LoadReview(resolvedStatePath(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Sources) != 1 || review.Sources[0].ID != "bb:5405" {
		t.Errorf("sources: got %+v, want one bb:5405", review.Sources)
	}
	count := 0
	for _, surface := range surfaces(review) {
		for _, comment := range surface.comments {
			if comment.SourceID() == "bb:5405" {
				count++
			}
		}
	}
	if count != 3 {
		t.Errorf("imported comments: got %d, want 3 (one copy each)", count)
	}
}

// debug logging names each request and the credential kind, and never the secret.
func TestFetchDebugLogRedactsTheCredential(t *testing.T) {
	dir := initRepoForAui(t)
	secret := "ATATTsupersecret"
	useFetchEnvironment(t, &replayDoer{routes: auiRoutes(t)}, map[string]string{"BITBUCKET_ACCESS_TOKEN": secret})
	runCLIInDir(t, dir, "start")

	code, out, errOut := runCLIInDir(t, dir, "fetch", "--debug", auiWebURL)

	if code != 0 {
		t.Fatalf("fetch failed: %s", errOut)
	}
	if strings.Contains(out+errOut, secret) {
		t.Fatal("the credential reached the output")
	}
	for _, expected := range []string{"level=DEBUG msg=request", auiComments, "access token (<redacted>)"} {
		if !strings.Contains(errOut, expected) {
			t.Errorf("debug log does not contain %q:\n%s", expected, errOut)
		}
	}
}

// INFO is the default: no request line is logged without `--debug`.
func TestFetchLogsAtInfoByDefault(t *testing.T) {
	dir := initRepoForAui(t)
	useFetchEnvironment(t, &replayDoer{routes: auiRoutes(t)}, nil)
	runCLIInDir(t, dir, "start")

	_, _, errOut := runCLIInDir(t, dir, "fetch", auiWebURL)

	if strings.Contains(errOut, "level=DEBUG") {
		t.Errorf("debug output without --debug:\n%s", errOut)
	}
	if !strings.Contains(errOut, `level=INFO msg="fetching pull request"`) {
		t.Errorf("no INFO progress line:\n%s", errOut)
	}
}

func TestFetchLogsCredentialWarnings(t *testing.T) {
	dir := initRepoForAui(t)
	useFetchEnvironment(t, &replayDoer{routes: auiRoutes(t)}, map[string]string{"BITBUCKET_EMAIL": "a@b.c"})
	runCLIInDir(t, dir, "start")

	_, _, errOut := runCLIInDir(t, dir, "fetch", auiWebURL)

	if !strings.Contains(errOut, "level=WARN") || !strings.Contains(errOut, "BITBUCKET_API_TOKEN") {
		t.Errorf("no warning about the half-configured credential:\n%s", errOut)
	}
}

// a normaliser defect producing a document the schema rejects fails the fetch, as
// a producer's malformed file fails an import.
func TestFetchedDocumentMeetsTheSchemaGate(t *testing.T) {
	result, err := fetchForeignDocument(context.Background(),
		fetchEnvironment{doer: &replayDoer{routes: auiRoutes(t)}, getenv: func(string) string { return "" }},
		fetchTarget{url: auiWebURL}, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := feedback.Encode(*result.doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := feedback.Decode(encoded); err != nil {
		t.Errorf("the fetched document fails the schema: %v", err)
	}
}

func TestFetchReportLines(t *testing.T) {
	given := fetched{
		doc: &feedback.Document{PullRequest: feedback.PullRequest{Source: feedback.Revision{Branch: "other"}}},
		findings: []feedback.Finding{
			{Severity: feedback.Debug, Code: feedback.DeletedCommentsPresent, Detail: "2 of 5"},
			{Severity: feedback.Warning, Code: feedback.OrphanReply, Detail: "reply 7"},
			{Severity: feedback.Error, Code: feedback.ReplyCycle, Detail: "comment 8"},
		},
	}

	actual := fetchReport(importReport{Source: "bb:1"}, given, "feature-x").Lines()

	expected := []string{
		"imported 0 comments from bb:1 (0 placed, 0 untethered, 0 replies)",
		`the pull request's source branch is "other" but the review is of "feature-x"`,
		"error reply_cycle: comment 8",
		"warning orphan_reply: reply 7",
	}
	if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(actual, "\n"), strings.Join(expected, "\n"))
	}
}

func TestFetchReportOmitsTheMismatchOnTheSameBranch(t *testing.T) {
	given := fetched{doc: &feedback.Document{PullRequest: feedback.PullRequest{Source: feedback.Revision{Branch: "feature-x"}}}}

	actual := fetchReport(importReport{Source: "bb:1"}, given, "feature-x")

	if actual.BranchMismatch != nil {
		t.Errorf("got a mismatch %+v on the same branch", actual.BranchMismatch)
	}
}

// the provider-independent package, which every import passes through, cannot reach
// the network: no package it depends on is `net` or `net/http`. Asserted over the
// import graph, so moving code between files cannot defeat it.
func TestFeedbackPackageHasNoNetworkDependency(t *testing.T) {
	output, err := exec.Command("go", "list", "-deps", "code-review/feedback").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for dependency := range strings.FieldsSeq(string(output)) {
		if dependency == "net" || dependency == "net/http" || strings.HasPrefix(dependency, "code-review/feedback/") {
			t.Errorf("code-review/feedback depends on %s", dependency)
		}
	}
}

// `fetch` is the only command that obtains the fetch environment, so it is the only
// one that can issue a request or read a credential.
func TestOnlyFetchReachesTheNetworkOrTheEnvironment(t *testing.T) {
	dir := initRepoForAui(t)
	runCLIInDir(t, dir, "start")
	document := writeFixtureDocument(t, dir)
	given := map[string][]string{
		"list": {"list"}, "status": {"status"}, "show": {"show", "missing"}, "resolve": {"resolve", "missing"},
		"reactivate": {"reactivate", "missing"}, "reply": {"reply", "missing", "x"}, "comment": {"comment", "x"},
		"unmark": {"unmark", "a.go"}, "import": {"import", document}, "instructions": {"instructions"},
		"schema": {"schema"}, "start": {"start"},
	}
	for name := range commands {
		if name == "fetch" {
			continue
		}
		args, covered := given[name]
		if !covered {
			t.Errorf("command %q has no case here: add one", name)
			continue
		}
		reached := false
		previous := currentFetchEnvironment
		currentFetchEnvironment = func() fetchEnvironment { reached = true; return previous() }
		runCLIInDir(t, dir, args...)
		currentFetchEnvironment = previous
		if reached {
			t.Errorf("%q obtained the fetch environment", name)
		}
	}
}

// an app over `dir` on `feature` against `main`, with its diff computed, saving to a
// state file under a temporary directory.
func fetchApp(t *testing.T, dir string) *App {
	t.Helper()
	state_dir := t.TempDir()
	app := &App{
		ctx:       context.Background(),
		review:    model.NewReview(dir, "feature", "main"),
		repoPath:  dir,
		dataDir:   state_dir,
		statePath: filepath.Join(state_dir, "review.json"),
	}
	if err := app.RecomputeDiff(); err != nil {
		t.Fatal(err)
	}
	if err := app.persist(); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestAppFetchForeignReviewPersistsAndReports(t *testing.T) {
	dir := initRepoForAui(t)
	useFetchEnvironment(t, &replayDoer{routes: auiRoutes(t, 5405)}, nil)
	app := fetchApp(t, dir)

	raw, err := app.FetchForeignReview("  ")

	if err != nil {
		t.Fatal(err)
	}
	var actual ImportResult
	if err := json.Unmarshal([]byte(raw), &actual); err != nil {
		t.Fatal(err)
	}
	if actual.Source != "bb:5405" || len(actual.Lines) == 0 || !strings.HasPrefix(actual.Lines[0], "imported 3 comments") {
		t.Errorf("got %+v", actual)
	}
	saved, err := LoadReview(app.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Sources) != 1 || saved.Sources[0].ID != "bb:5405" {
		t.Errorf("persisted sources: got %+v", saved.Sources)
	}
}

func TestAppFetchForeignReviewFailureLeavesTheReviewUnchanged(t *testing.T) {
	dir := initRepoForAui(t)
	useFetchEnvironment(t, &replayDoer{routes: map[string]replayReply{}}, nil)
	app := fetchApp(t, dir)
	before, err := os.ReadFile(app.statePath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = app.FetchForeignReview(auiWebURL)

	if err == nil || !strings.Contains(err.Error(), "not visible") {
		t.Errorf("got %v, want a not-found error", err)
	}
	after, err := os.ReadFile(app.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || len(app.review.Sources) != 0 {
		t.Error("a failed fetch changed the review")
	}
}
