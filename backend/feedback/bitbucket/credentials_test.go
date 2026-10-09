package bitbucket

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// a lookup over `pairs`, standing in for the process environment.
func env(pairs map[string]string) func(string) string {
	return func(name string) string { return pairs[name] }
}

func TestResolveCredentialsKinds(t *testing.T) {
	given := []struct {
		name     string
		pairs    map[string]string
		expected credentialKind
		warnings int
	}{
		{"access token alone", map[string]string{AccessTokenVar: "tok"}, accessToken, 0},
		{"api token pair", map[string]string{EmailVar: "a@b.c", APITokenVar: "tok"}, apiToken, 0},
		{"access token takes precedence", map[string]string{AccessTokenVar: "b", EmailVar: "a@b.c", APITokenVar: "c"}, accessToken, 1},
		{"email without token", map[string]string{EmailVar: "a@b.c"}, anonymous, 1},
		{"token without email", map[string]string{APITokenVar: "tok"}, anonymous, 1},
		{"nothing set", map[string]string{}, anonymous, 0},
		{"empty access token", map[string]string{AccessTokenVar: ""}, anonymous, 0},
		{"whitespace api token", map[string]string{EmailVar: "a@b.c", APITokenVar: "   "}, anonymous, 1},
	}
	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			actual, warnings := ResolveCredentials(env(test.pairs))
			if actual.kind != test.expected {
				t.Errorf("kind: got %v, want %v", actual.kind, test.expected)
			}
			if len(warnings) != test.warnings {
				t.Errorf("warnings: got %q, want %d", warnings, test.warnings)
			}
		})
	}
}

// a warning about half a pair names the variable that is missing.
func TestPartialPairWarningNamesTheMissingVariable(t *testing.T) {
	_, warnings := ResolveCredentials(env(map[string]string{EmailVar: "a@b.c"}))
	if len(warnings) != 1 || !strings.Contains(warnings[0], APITokenVar) {
		t.Errorf("got %q, want a warning naming %s", warnings, APITokenVar)
	}
}

// no rendering of resolved credentials, and no warning, carries the secret.
func TestNoRenderingDisclosesTheSecret(t *testing.T) {
	secret := "ATATTverysecret"
	given := []map[string]string{
		{AccessTokenVar: secret},
		{EmailVar: "a@b.c", APITokenVar: secret},
		{AccessTokenVar: secret, EmailVar: "a@b.c", APITokenVar: secret},
		{APITokenVar: secret},
	}
	for _, pairs := range given {
		credentials, warnings := ResolveCredentials(env(pairs))
		rendered := fmt.Sprintf("%v %+v %#v %s %q", credentials, credentials, credentials, credentials, credentials) +
			credentials.Redacted() + strings.Join(warnings, " ")
		if strings.Contains(rendered, secret) {
			t.Errorf("secret disclosed in %q", rendered)
		}
	}
}

func TestAuthorize(t *testing.T) {
	given := []struct {
		name     string
		pairs    map[string]string
		expected string
	}{
		{"bearer", map[string]string{AccessTokenVar: "tok"}, "Bearer tok"},
		// base64("a@b.c:tok")
		{"basic", map[string]string{EmailVar: "a@b.c", APITokenVar: "tok"}, "Basic YUBiLmM6dG9r"},
		{"anonymous", map[string]string{}, ""},
	}
	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			credentials, _ := ResolveCredentials(env(test.pairs))
			request, _ := http.NewRequest(http.MethodGet, "https://api.bitbucket.org/2.0", nil)
			credentials.authorize(request)
			if actual := request.Header.Get("Authorization"); actual != test.expected {
				t.Errorf("got %q, want %q", actual, test.expected)
			}
		})
	}
}
