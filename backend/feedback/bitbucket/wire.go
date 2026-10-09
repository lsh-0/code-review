package bitbucket

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// types shaped like the Bitbucket Cloud REST 2.0 payload, including its awkward
// parts: an inline anchor as four independent nullable integers, a deletion flag
// beside a body string, a parent reference present only on replies. Refining them
// is the normaliser's job. Nothing outside this package sees these types.
//
// Decoding ignores unknown keys, so additions to the API do not break retrieval.
// An absent field and an explicit `null` mean the same. A value of the wrong type
// is an error, never a default.

// a user as the API reports it. Neither `AccountID` nor `UUID` is guaranteed
// present, so neither can serve as an identity alone. `Kind` accompanies an
// application user, with values such as `workspace_access_token`.
type wireUser struct {
	DisplayName string
	AccountID   *string
	UUID        *string
	Nickname    *string
	Type        string // `user` or `app_user`, retained verbatim
	Kind        *string
}

// an inline anchor. `From` indexes the pre-image and `To` the post-image. Both
// absent means the comment addresses the whole file. A `Start` field makes the
// anchor a range ending at the corresponding line.
type wireInline struct {
	Path      string
	From      *int
	To        *int
	StartFrom *int
	StartTo   *int
}

// thread resolution as three distinct wire states. The API returns `{}` unless the
// request carries the field expansion, so an empty object means resolved with by
// whom and when withheld.
type resolutionState int

const (
	resolutionAbsent resolutionState = iota
	resolutionEmpty
	resolutionDetailed
)

type wireResolution struct {
	State     resolutionState
	CreatedOn string
	User      wireUser
}

type wireComment struct {
	ID         int
	CreatedOn  string // RFC 3339 text, parsed by the normaliser so a bad value is a finding
	UpdatedOn  *string
	BodyRaw    string // `content.raw`, empty for a deleted comment
	Markup     *string
	User       wireUser
	Deleted    bool
	Pending    bool
	ParentID   *int
	Inline     *wireInline
	Resolution wireResolution
}

type wireParticipant struct {
	User     wireUser
	Role     string
	Approved bool
	State    *string
}

type wireRevision struct {
	Branch string
	Commit *string
}

type wirePullRequest struct {
	ID           int
	Title        string
	Description  string
	State        string // retained verbatim, so an unknown value is a finding and not a failure
	Draft        bool
	Author       wireUser
	CreatedOn    string
	UpdatedOn    *string
	Source       wireRevision
	Destination  wireRevision
	Participants []wireParticipant
	CommentCount int
	TaskCount    int
	// the web URL, from `links.html.href`, for naming a pull request in an error.
	HTMLURL string
}

// one page of a cursor-paginated collection. `Next` is the absolute URL of the
// following page, absent on the last one.
type wirePage[T any] struct {
	Values []T
	Next   *string
	Size   *int
}

// a field read from an object: its raw value, or absent. `null` reads as absent.
type fields map[string]json.RawMessage

func objectFields(data []byte, at string) (fields, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("%s: expected an object: %w", at, err)
	}
	if object == nil {
		return nil, fmt.Errorf("%s: expected an object, got null", at)
	}
	return object, nil
}

// the raw value at `key`, and false when it is absent or `null`.
func (f fields) get(key string) (json.RawMessage, bool) {
	raw, present := f[key]
	if !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, false
	}
	return raw, true
}

// decodes the value at `key` into `out`, failing when it is absent.
func required[T any](f fields, key string, at string, out *T) error {
	raw, present := f.get(key)
	if !present {
		return fmt.Errorf("%s.%s: required, got nothing", at, key)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s.%s: %w", at, key, err)
	}
	return nil
}

// decodes the value at `key` into a new value, or returns nil when it is absent.
func optional[T any](f fields, key string, at string) (*T, error) {
	raw, present := f.get(key)
	if !present {
		return nil, nil
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("%s.%s: %w", at, key, err)
	}
	return &value, nil
}

// decodes the value at `key`, or returns `fallback` when it is absent.
func withDefault[T any](f fields, key string, at string, fallback T) (T, error) {
	value, err := optional[T](f, key, at)
	if err != nil || value == nil {
		return fallback, err
	}
	return *value, nil
}

// the raw object at `key`, or nil when absent.
func nested(f fields, key string, at string) (fields, error) {
	raw, present := f.get(key)
	if !present {
		return nil, nil
	}
	return objectFields(raw, at+"."+key)
}

func decodeUser(data []byte, at string) (wireUser, error) {
	f, err := objectFields(data, at)
	if err != nil {
		return wireUser{}, err
	}
	var user wireUser
	if err := required(f, "display_name", at, &user.DisplayName); err != nil {
		return wireUser{}, err
	}
	if user.AccountID, err = optional[string](f, "account_id", at); err != nil {
		return wireUser{}, err
	}
	if user.UUID, err = optional[string](f, "uuid", at); err != nil {
		return wireUser{}, err
	}
	if user.Nickname, err = optional[string](f, "nickname", at); err != nil {
		return wireUser{}, err
	}
	if user.Type, err = withDefault(f, "type", at, "user"); err != nil {
		return wireUser{}, err
	}
	if user.Kind, err = optional[string](f, "kind", at); err != nil {
		return wireUser{}, err
	}
	return user, nil
}

func requiredUser(f fields, key string, at string) (wireUser, error) {
	raw, present := f.get(key)
	if !present {
		return wireUser{}, fmt.Errorf("%s.%s: required, got nothing", at, key)
	}
	return decodeUser(raw, at+"."+key)
}

func decodeInline(f fields, at string) (*wireInline, error) {
	if f == nil {
		return nil, nil
	}
	var inline wireInline
	var err error
	if err = required(f, "path", at, &inline.Path); err != nil {
		return nil, err
	}
	if inline.From, err = optional[int](f, "from", at); err != nil {
		return nil, err
	}
	if inline.To, err = optional[int](f, "to", at); err != nil {
		return nil, err
	}
	if inline.StartFrom, err = optional[int](f, "start_from", at); err != nil {
		return nil, err
	}
	if inline.StartTo, err = optional[int](f, "start_to", at); err != nil {
		return nil, err
	}
	return &inline, nil
}

// `{}` and a populated object are different states, so emptiness is checked before
// the detail fields are read.
func decodeResolution(f fields, at string) (wireResolution, error) {
	if f == nil {
		return wireResolution{State: resolutionAbsent}, nil
	}
	if len(f) == 0 {
		return wireResolution{State: resolutionEmpty}, nil
	}
	resolution := wireResolution{State: resolutionDetailed}
	if err := required(f, "created_on", at, &resolution.CreatedOn); err != nil {
		return wireResolution{}, err
	}
	user, err := requiredUser(f, "user", at)
	if err != nil {
		return wireResolution{}, err
	}
	resolution.User = user
	return resolution, nil
}

func decodeComment(data []byte, at string) (wireComment, error) {
	f, err := objectFields(data, at)
	if err != nil {
		return wireComment{}, err
	}
	var comment wireComment
	if err := required(f, "id", at, &comment.ID); err != nil {
		return wireComment{}, err
	}
	if err := required(f, "created_on", at, &comment.CreatedOn); err != nil {
		return wireComment{}, err
	}
	if comment.UpdatedOn, err = optional[string](f, "updated_on", at); err != nil {
		return wireComment{}, err
	}

	content, err := nested(f, "content", at)
	if err != nil {
		return wireComment{}, err
	}
	if content == nil {
		return wireComment{}, fmt.Errorf("%s.content: required, got nothing", at)
	}
	if comment.BodyRaw, err = withDefault(content, "raw", at+".content", ""); err != nil {
		return wireComment{}, err
	}
	if comment.Markup, err = optional[string](content, "markup", at+".content"); err != nil {
		return wireComment{}, err
	}

	if comment.User, err = requiredUser(f, "user", at); err != nil {
		return wireComment{}, err
	}
	if comment.Deleted, err = withDefault(f, "deleted", at, false); err != nil {
		return wireComment{}, err
	}
	if comment.Pending, err = withDefault(f, "pending", at, false); err != nil {
		return wireComment{}, err
	}

	parent, err := nested(f, "parent", at)
	if err != nil {
		return wireComment{}, err
	}
	if parent != nil {
		var id int
		if err := required(parent, "id", at+".parent", &id); err != nil {
			return wireComment{}, err
		}
		comment.ParentID = &id
	}

	inline, err := nested(f, "inline", at)
	if err != nil {
		return wireComment{}, err
	}
	if comment.Inline, err = decodeInline(inline, at+".inline"); err != nil {
		return wireComment{}, err
	}

	resolution, err := nested(f, "resolution", at)
	if err != nil {
		return wireComment{}, err
	}
	if comment.Resolution, err = decodeResolution(resolution, at+".resolution"); err != nil {
		return wireComment{}, err
	}
	return comment, nil
}

func decodeRevision(f fields, key string, at string) (wireRevision, error) {
	object, err := nested(f, key, at)
	if err != nil {
		return wireRevision{}, err
	}
	if object == nil {
		return wireRevision{}, fmt.Errorf("%s.%s: required, got nothing", at, key)
	}
	at = at + "." + key
	var revision wireRevision
	branch, err := nested(object, "branch", at)
	if err != nil {
		return wireRevision{}, err
	}
	if branch == nil {
		return wireRevision{}, fmt.Errorf("%s.branch: required, got nothing", at)
	}
	if revision.Branch, err = withDefault(branch, "name", at+".branch", ""); err != nil {
		return wireRevision{}, err
	}
	commit, err := nested(object, "commit", at)
	if err != nil {
		return wireRevision{}, err
	}
	if commit != nil {
		var hash string
		if err := required(commit, "hash", at+".commit", &hash); err != nil {
			return wireRevision{}, err
		}
		revision.Commit = &hash
	}
	return revision, nil
}

func decodeParticipant(data []byte, at string) (wireParticipant, error) {
	f, err := objectFields(data, at)
	if err != nil {
		return wireParticipant{}, err
	}
	var participant wireParticipant
	if participant.User, err = requiredUser(f, "user", at); err != nil {
		return wireParticipant{}, err
	}
	if participant.Role, err = withDefault(f, "role", at, "PARTICIPANT"); err != nil {
		return wireParticipant{}, err
	}
	if participant.Approved, err = withDefault(f, "approved", at, false); err != nil {
		return wireParticipant{}, err
	}
	if participant.State, err = optional[string](f, "state", at); err != nil {
		return wireParticipant{}, err
	}
	return participant, nil
}

// decodes each element of the array at `key` with `decode`, or returns an empty
// list when it is absent.
func list[T any](f fields, key string, at string, decode func([]byte, string) (T, error)) ([]T, error) {
	raw, err := withDefault(f, key, at, []json.RawMessage{})
	if err != nil {
		return nil, err
	}
	values := make([]T, 0, len(raw))
	for i, element := range raw {
		value, err := decode(element, fmt.Sprintf("%s.%s[%d]", at, key, i))
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func decodePullRequest(data []byte) (wirePullRequest, error) {
	const at = "pull_request"
	f, err := objectFields(data, at)
	if err != nil {
		return wirePullRequest{}, err
	}
	var pr wirePullRequest
	if err := required(f, "id", at, &pr.ID); err != nil {
		return wirePullRequest{}, err
	}
	if pr.Title, err = withDefault(f, "title", at, ""); err != nil {
		return wirePullRequest{}, err
	}
	if pr.Description, err = withDefault(f, "description", at, ""); err != nil {
		return wirePullRequest{}, err
	}
	if pr.State, err = withDefault(f, "state", at, ""); err != nil {
		return wirePullRequest{}, err
	}
	if pr.Draft, err = withDefault(f, "draft", at, false); err != nil {
		return wirePullRequest{}, err
	}
	if pr.Author, err = requiredUser(f, "author", at); err != nil {
		return wirePullRequest{}, err
	}
	if err := required(f, "created_on", at, &pr.CreatedOn); err != nil {
		return wirePullRequest{}, err
	}
	if pr.UpdatedOn, err = optional[string](f, "updated_on", at); err != nil {
		return wirePullRequest{}, err
	}
	if pr.Source, err = decodeRevision(f, "source", at); err != nil {
		return wirePullRequest{}, err
	}
	if pr.Destination, err = decodeRevision(f, "destination", at); err != nil {
		return wirePullRequest{}, err
	}
	if pr.Participants, err = list(f, "participants", at, decodeParticipant); err != nil {
		return wirePullRequest{}, err
	}
	if pr.CommentCount, err = withDefault(f, "comment_count", at, 0); err != nil {
		return wirePullRequest{}, err
	}
	if pr.TaskCount, err = withDefault(f, "task_count", at, 0); err != nil {
		return wirePullRequest{}, err
	}
	pr.HTMLURL = htmlLink(f)
	return pr, nil
}

// `links.html.href`, or empty. Read leniently: it names a pull request in an error
// and never decides anything.
func htmlLink(f fields) string {
	var links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	}
	if raw, present := f.get("links"); present {
		_ = json.Unmarshal(raw, &links)
	}
	return links.HTML.Href
}

// decodes one page whose values are decoded with `decode`.
func decodePage[T any](data []byte, decode func([]byte, string) (T, error)) (wirePage[T], error) {
	const at = "page"
	f, err := objectFields(data, at)
	if err != nil {
		return wirePage[T]{}, err
	}
	if _, present := f.get("values"); !present {
		return wirePage[T]{}, fmt.Errorf("%s.values: required, got nothing", at)
	}
	var page wirePage[T]
	if page.Values, err = list(f, "values", at, decode); err != nil {
		return wirePage[T]{}, err
	}
	if page.Next, err = optional[string](f, "next", at); err != nil {
		return wirePage[T]{}, err
	}
	if page.Size, err = optional[int](f, "size", at); err != nil {
		return wirePage[T]{}, err
	}
	return page, nil
}

// decodes a comments payload saved either as a page envelope or as a bare array of
// comments, so a fixture replays whichever way it was captured.
func decodeComments(data []byte) ([]wireComment, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var raw []json.RawMessage
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return nil, fmt.Errorf("comments: %w", err)
		}
		comments := make([]wireComment, 0, len(raw))
		for i, element := range raw {
			comment, err := decodeComment(element, fmt.Sprintf("comments[%d]", i))
			if err != nil {
				return nil, err
			}
			comments = append(comments, comment)
		}
		return comments, nil
	}
	page, err := decodePage(data, decodeComment)
	if err != nil {
		return nil, err
	}
	return page.Values, nil
}
