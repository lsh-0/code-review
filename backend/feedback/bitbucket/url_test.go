package bitbucket

import (
	"net/url"
	"strings"
	"testing"
)

var aui = PullRequestRef{Repository: Repository{Workspace: "atlassian", Slug: "aui"}, Number: 5405}

func TestPullRequestURL(t *testing.T) {
	expected := "https://api.bitbucket.org/2.0/repositories/atlassian/aui/pullrequests/5405"
	actual := pullRequestURL(aui)
	if actual != expected {
		t.Errorf("got %q, want %q", actual, expected)
	}
}

func TestPercentEncode(t *testing.T) {
	given := []struct{ in, expected string }{
		{"a b/c", "a%20b%2Fc"},
		{"Az09-_.~", "Az09-_.~"},
	}
	for _, test := range given {
		if actual := percentEncode(test.in); actual != test.expected {
			t.Errorf("percentEncode(%q) = %q, want %q", test.in, actual, test.expected)
		}
	}

	odd := aui
	odd.Slug = "my repo"
	if !strings.Contains(pullRequestURL(odd), "/my%20repo/") {
		t.Errorf("an encoded slug does not appear encoded in the path: %q", pullRequestURL(odd))
	}
}

// the field expansion is asserted explicitly: omitting it silently reduces every
// resolved thread to the detail-less case.
func TestCommentsURLCarriesPageSizeAndExpansion(t *testing.T) {
	actual := commentsURL(aui)
	for _, needle := range []string{"/comments?", "pagelen=100", "fields=%2Bvalues.resolution.*"} {
		if !strings.Contains(actual, needle) {
			t.Errorf("%q does not contain %q", actual, needle)
		}
	}
}

// a branch name cannot change the meaning of the query it is placed in.
func TestOpenPullRequestsURLQuotesTheBranch(t *testing.T) {
	given := `fix/a "quoted" \ branch`

	actual, err := url.Parse(openPullRequestsURL(aui.Repository, given))
	if err != nil {
		t.Fatal(err)
	}

	expected := `source.branch.name="fix/a \"quoted\" \\ branch"`
	if q := actual.Query().Get("q"); q != expected {
		t.Errorf("q = %q, want %q", q, expected)
	}
	if state := actual.Query().Get("state"); state != "OPEN" {
		t.Errorf("state = %q, want OPEN", state)
	}
	if actual.Host != apiHost {
		t.Errorf("host = %q, want %q", actual.Host, apiHost)
	}
}

func TestParsePullRequestURL(t *testing.T) {
	given := []string{
		"https://bitbucket.org/atlassian/aui/pull-requests/5405",
		"https://bitbucket.org/atlassian/aui/pull-requests/5405/",
		"https://bitbucket.org/atlassian/aui/pull-requests/5405/overview",
		"https://bitbucket.org/atlassian/aui/pull-requests/5405/diff#chg-a.go",
		"https://bitbucket.org/atlassian/aui/pull-requests/5405/commits?w=1",
		"  https://www.bitbucket.org/atlassian/aui/pull-requests/5405  ",
	}
	for _, raw := range given {
		actual, err := ParsePullRequestURL(raw)
		if err != nil {
			t.Errorf("%q: %v", raw, err)
			continue
		}
		if actual != aui {
			t.Errorf("%q: got %+v, want %+v", raw, actual, aui)
		}
	}
}

func TestParsePullRequestURLRejects(t *testing.T) {
	given := []struct {
		raw     string
		message string
	}{
		{"https://bitbucket.org/atlassian/aui/pull-requests/", "does not name a pull request"},
		{"https://bitbucket.org/atlassian/aui/pull-requests/abc", "number"},
		{"https://bitbucket.org/atlassian/aui/pull-requests/0", "number"},
		{"https://bitbucket.org/atlassian/aui/src/master", "does not name a pull request"},
		{"https://github.com/atlassian/aui/pull/5405", `unsupported pull request host "github.com"`},
		{"bitbucket.org/atlassian/aui/pull-requests/5405", "not a pull request URL"},
		{"ftp://bitbucket.org/atlassian/aui/pull-requests/5405", "not a pull request URL"},
		{"5405", "not a pull request URL"},
	}
	for _, test := range given {
		_, err := ParsePullRequestURL(test.raw)
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("%q: got %v, want an error containing %q", test.raw, err, test.message)
			continue
		}
		if !strings.Contains(err.Error(), strings.TrimSpace(test.raw)) {
			t.Errorf("%q: error %q does not quote the URL", test.raw, err)
		}
	}
}

func TestParseRemote(t *testing.T) {
	expected := Repository{Workspace: "acme", Slug: "router"}
	given := []string{
		"git@bitbucket.org:acme/router.git",
		"git@bitbucket.org:acme/router",
		"ssh://git@bitbucket.org/acme/router.git",
		"ssh://git@bitbucket.org:22/acme/router.git",
		"https://bitbucket.org/acme/router.git",
		"https://someone@bitbucket.org/acme/router",
		"https://bitbucket.org/acme/router/",
	}
	for _, raw := range given {
		actual, err := ParseRemote(raw)
		if err != nil {
			t.Errorf("%q: %v", raw, err)
			continue
		}
		if actual != expected {
			t.Errorf("%q: got %+v, want %+v", raw, actual, expected)
		}
	}
}

func TestParseRemoteRejects(t *testing.T) {
	given := []struct {
		raw     string
		message string
	}{
		{"git@github.com:lsh-0/code-review.git", `on "github.com"`},
		{"https://github.com/lsh-0/code-review", `on "github.com"`},
		{"git@bitbucket.org:acme.git", "does not name a workspace and repository"},
		{"git@bitbucket.org:acme/router/extra.git", "does not name a workspace and repository"},
		{"/srv/git/router.git", "not a recognised git remote URL"},
		{"", "not a recognised git remote URL"},
	}
	for _, test := range given {
		_, err := ParseRemote(test.raw)
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("%q: got %v, want an error containing %q", test.raw, err, test.message)
		}
	}
}

func TestWebURL(t *testing.T) {
	expected := "https://bitbucket.org/atlassian/aui/pull-requests/5405"
	if actual := aui.WebURL(); actual != expected {
		t.Errorf("got %q, want %q", actual, expected)
	}
}

// no input panics either parser, and an accepted pull request URL describes the same
// pull request when rebuilt from what was read.
func FuzzParsePullRequestURL(f *testing.F) {
	f.Add("https://bitbucket.org/atlassian/aui/pull-requests/5405/overview")
	f.Add("https://bitbucket.org/a/b/pull-requests/1")
	f.Add("https://bitbucket.org//pull-requests/")
	f.Add("http://bitbucket.org:99999/a/b/pull-requests/1")
	f.Fuzz(func(t *testing.T, raw string) {
		given, err := ParsePullRequestURL(raw)
		if err != nil {
			return
		}
		if given.Workspace == "" || given.Slug == "" || given.Number < 1 {
			t.Fatalf("accepted %q as an incomplete reference %+v", raw, given)
		}
		actual, err := ParsePullRequestURL(given.WebURL())
		if err != nil {
			t.Fatalf("rebuilt URL %q for %q is rejected: %v", given.WebURL(), raw, err)
		}
		if actual != given {
			t.Fatalf("round trip of %q: got %+v, want %+v", raw, actual, given)
		}
	})
}

func FuzzParseRemote(f *testing.F) {
	f.Add("git@bitbucket.org:acme/router.git")
	f.Add("ssh://git@bitbucket.org/acme/router.git")
	f.Add("https://bitbucket.org/acme/router")
	f.Add(":/")
	f.Fuzz(func(t *testing.T, raw string) {
		given, err := ParseRemote(raw)
		if err != nil {
			return
		}
		if given.Workspace == "" || given.Slug == "" {
			t.Fatalf("accepted %q as an incomplete repository %+v", raw, given)
		}
	})
}
