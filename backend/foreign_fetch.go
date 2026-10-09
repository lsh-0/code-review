package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"code-review/feedback"
	"code-review/feedback/bitbucket"
)

// retrieves review feedback from a hosted pull request and turns it into an import
// document, which then merges through the same path as `import <file>`.
//
// This is the only network access the program has. It is reached from the `fetch`
// command and the GUI's fetch control, and from nowhere else.

// what a fetch reaches the outside world through: the HTTP client, and the
// environment credentials are read from. Production uses `defaultFetchEnvironment`.
// A test substitutes both, so no test opens a socket or reads the real environment.
type fetchEnvironment struct {
	doer   bitbucket.Doer
	getenv func(string) string
}

// the time one request may take, including reading its body.
const fetchRequestTimeout = 30 * time.Second

func defaultFetchEnvironment() fetchEnvironment {
	return fetchEnvironment{
		doer:   &http.Client{Timeout: fetchRequestTimeout},
		getenv: os.Getenv,
	}
}

// the fetch environment commands use. A variable only so tests can substitute it.
var currentFetchEnvironment = defaultFetchEnvironment

// which pull request to fetch: one named by URL, or the open pull request whose
// source branch is the review's, found through the repository's `origin` remote.
type fetchTarget struct {
	url          string
	repoPath     string
	sourceBranch string
}

// the outcome of a fetch: the document, already through the schema gate, the
// findings at every severity, and where it came from.
type fetched struct {
	doc      *feedback.Document
	findings []feedback.Finding
	ref      bitbucket.PullRequestRef
}

// resolves `target` to a pull request reference. Discovery reads `origin` and asks
// the API for the open pull request on the source branch.
func locate(ctx context.Context, client bitbucket.Client, target fetchTarget) (bitbucket.PullRequestRef, error) {
	if target.url != "" {
		return bitbucket.ParsePullRequestURL(target.url)
	}
	remote, err := GetRemoteURL(target.repoPath, "origin")
	if err != nil {
		return bitbucket.PullRequestRef{}, fmt.Errorf("finding the pull request needs a Bitbucket `origin` remote (%v); give a pull request URL instead", err)
	}
	repo, err := bitbucket.ParseRemote(remote)
	if err != nil {
		return bitbucket.PullRequestRef{}, fmt.Errorf("finding the pull request needs a Bitbucket `origin` remote: %v; give a pull request URL instead", err)
	}
	return client.FindOpenForBranch(ctx, repo, target.sourceBranch)
}

// locates, retrieves, normalises and validates one pull request's feedback.
//
// The normalised document is encoded and decoded again, so it meets the schema gate
// a document from a file meets. A normaliser defect then fails as an external
// producer's would, and never reaches the review. Nothing is written here.
func fetchForeignDocument(ctx context.Context, env fetchEnvironment, target fetchTarget, log *slog.Logger) (fetched, error) {
	credentials, warnings := bitbucket.ResolveCredentials(env.getenv)
	for _, warning := range warnings {
		log.Warn(warning)
	}
	client := bitbucket.Client{
		Doer:        env.doer,
		Credentials: credentials,
		Log: func(url string, as string) {
			log.Debug("request", "method", http.MethodGet, "url", url, "credentials", as)
		},
	}

	ref, err := locate(ctx, client, target)
	if err != nil {
		return fetched{}, err
	}
	log.Info("fetching pull request", "url", ref.WebURL(), "credentials", credentials.Redacted())

	doc, findings, err := bitbucket.Fetch(ctx, client, ref)
	if err != nil {
		return fetched{}, err
	}
	findings = feedback.Sort(append(findings, feedback.Check(doc)...))
	log.Info("retrieved feedback", "comments", len(feedback.Comments(doc)), "threads", len(doc.Threads()))
	for _, finding := range findings {
		if finding.Severity == feedback.Debug {
			log.Debug("finding", "code", string(finding.Code), "detail", finding.Detail)
		}
	}

	encoded, err := feedback.Encode(doc)
	if err != nil {
		return fetched{}, fmt.Errorf("failed to encode the fetched feedback: %w", err)
	}
	checked, err := feedback.Decode(encoded)
	if err != nil {
		return fetched{}, fmt.Errorf("the fetched feedback is not a valid import document: %w", err)
	}
	return fetched{doc: checked, findings: findings, ref: ref}, nil
}

// the report for a fetch: the import report, with the findings worth a reviewer's
// attention and any branch mismatch.
func fetchReport(report importReport, result fetched, reviewBranch string) importReport {
	report.Findings = feedback.AtLeast(result.findings, feedback.Warning)
	if branch := result.doc.PullRequest.Source.Branch; branch != reviewBranch {
		report.BranchMismatch = &branchMismatch{PullRequest: branch, Review: reviewBranch}
	}
	return report
}
