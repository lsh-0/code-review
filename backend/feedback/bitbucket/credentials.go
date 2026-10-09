package bitbucket

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// the environment variables credentials are read from. Credentials are never
// accepted as arguments: arguments appear in shell history and in `ps` output.
const (
	AccessTokenVar = "BITBUCKET_ACCESS_TOKEN"
	EmailVar       = "BITBUCKET_EMAIL"
	APITokenVar    = "BITBUCKET_API_TOKEN"
)

type credentialKind int

const (
	anonymous credentialKind = iota
	apiToken
	accessToken
)

// how requests authenticate. The fields are unexported and every rendering is
// redacted, so a secret cannot reach a log, an error or a report through `fmt`.
//
//   - anonymous: no `Authorization` header. Enough for a public repository.
//   - API token: an Atlassian account API token with Bitbucket scopes, paired with
//     the account email, sent as HTTP Basic.
//   - access token: a repository, project or workspace access token, sent as a
//     bearer token.
//
// The zero value is anonymous.
type Credentials struct {
	kind  credentialKind
	email string
	token string
}

// the credential kind with the secret replaced by a marker. The only rendering of
// credentials this package offers.
func (c Credentials) Redacted() string {
	switch c.kind {
	case apiToken:
		return fmt.Sprintf("api token for %s (<redacted>)", c.email)
	case accessToken:
		return "access token (<redacted>)"
	default:
		return "anonymous"
	}
}

// redacted, so `%v` and `%s` cannot print the secret.
func (c Credentials) String() string {
	return c.Redacted()
}

// redacted, so `%#v` cannot print the secret.
func (c Credentials) GoString() string {
	return "bitbucket.Credentials{" + c.Redacted() + "}"
}

// sets the `Authorization` header for these credentials. Anonymous sets nothing.
func (c Credentials) authorize(request *http.Request) {
	switch c.kind {
	case apiToken:
		pair := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.token))
		request.Header.Set("Authorization", "Basic "+pair)
	case accessToken:
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// a value that is set and not blank. An exported-but-empty variable is a common
// shell accident and is not a credential.
func nonBlank(value string) (string, bool) {
	return value, strings.TrimSpace(value) != ""
}

// resolves credentials from `getenv`, returning them with a warning for each piece of
// configuration that was ignored. Pure: the environment is passed in, never read.
//
// `BITBUCKET_ACCESS_TOKEN` takes precedence and is sent as a bearer token. Otherwise
// `BITBUCKET_EMAIL` with `BITBUCKET_API_TOKEN` is sent as HTTP Basic. Otherwise the
// result is anonymous.
//
// Half a Basic pair resolves to anonymous with a warning naming both variables.
// Proceeding anonymously while silently dropping half a credential would present a
// 404 as though the repository did not exist.
//
// App passwords are not supported. Bitbucket removed them on 28 July 2026.
func ResolveCredentials(getenv func(string) string) (Credentials, []string) {
	token, hasToken := nonBlank(getenv(AccessTokenVar))
	email, hasEmail := nonBlank(getenv(EmailVar))
	secret, hasSecret := nonBlank(getenv(APITokenVar))

	switch {
	case hasToken:
		warnings := []string{}
		if hasEmail || hasSecret {
			warnings = append(warnings, fmt.Sprintf("%s is set, so %s and %s are ignored", AccessTokenVar, EmailVar, APITokenVar))
		}
		return Credentials{kind: accessToken, token: token}, warnings
	case hasEmail && hasSecret:
		return Credentials{kind: apiToken, email: email, token: secret}, []string{}
	case hasEmail:
		return Credentials{}, []string{fmt.Sprintf(
			"%s is set but %s is not, so it is ignored and the request is anonymous", EmailVar, APITokenVar)}
	case hasSecret:
		return Credentials{}, []string{fmt.Sprintf(
			"%s is set but %s is not, so it is ignored and the request is anonymous", APITokenVar, EmailVar)}
	default:
		return Credentials{}, []string{}
	}
}
