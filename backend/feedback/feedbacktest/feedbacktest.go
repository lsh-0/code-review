// Package feedbacktest builds import documents for tests: a minimal valid document
// that a test varies in one place, so each case states only what it is about.
//
// Every builder produces JSON text and not a `feedback.Document`, because what is
// under test is usually the decoder or the importer reading text a producer wrote.
package feedbacktest

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// the committed sample of upstream `bb-feedback` output, as an absolute path so a
// test in any package can read it.
var Fixture1430 = testdataPath("foreign_review_1430.json")

// an absolute path under `backend/testdata`.
func testdataPath(name string) string {
	_, here, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(here), "..", "..", "testdata", name)
}

// the path of a file under `backend/testdata`, as an absolute path.
func Testdata(name string) string {
	return testdataPath(name)
}

const MinimalPullRequest = `{
	"id": 1430,
	"title": "A title",
	"description": null,
	"state": "open",
	"draft": false,
	"author": {"display_name": "Luke", "account_id": "a", "uuid": "u", "nickname": "Luke", "kind": "human"},
	"created_at": "2026-09-04T03:54:57.776968Z",
	"updated_at": null,
	"source": {"branch": "feature", "commit": "abc123"},
	"destination": {"branch": "master", "commit": "def456"},
	"participants": [],
	"reported_comment_count": 0,
	"reported_task_count": 0
}`

const HumanAuthor = `{"display_name": "Luke", "account_id": "a", "uuid": "u", "nickname": "Luke", "kind": "human"}`

// the timestamp every built comment carries.
const CreatedAt = "2026-09-04T03:59:34.961609Z"

func MarkdownBody(text string) string {
	return `{"kind": "markdown", "text": "` + text + `"}`
}

func CommentJSON(id int, body string, pending bool, replies []string) string {
	return CommentByJSON(id, HumanAuthor, body, pending, replies)
}

// as `CommentJSON`, written by `author`.
func CommentByJSON(id int, author string, body string, pending bool, replies []string) string {
	return `{
		"id": ` + strconv.Itoa(id) + `,
		"author": ` + author + `,
		"body": ` + body + `,
		"created_at": "` + CreatedAt + `",
		"updated_at": null,
		"pending": ` + strconv.FormatBool(pending) + `,
		"replies": [` + strings.Join(replies, ",") + `]
	}`
}

func ThreadJSON(path string, position string, root string) string {
	return `{
		"location": {"kind": "file_revision", "path": "` + path + `", "position": ` + position + `},
		"resolution": {"state": "unresolved"},
		"root": ` + root + `
	}`
}

// a review-level thread with `resolution` and `root`.
func DiscussionThreadJSON(resolution string, root string) string {
	return `{
		"location": {"kind": "pull_request"},
		"resolution": ` + resolution + `,
		"root": ` + root + `
	}`
}

func FileThreadRaw(path string, position string) string {
	return ThreadJSON(path, position, CommentJSON(1, MarkdownBody("a finding"), false, nil))
}

func FileThread(path string, kind string, first int, last int) string {
	position := `{"kind": "` + kind + `", "lines": {"first": ` + strconv.Itoa(first) + `, "last": ` + strconv.Itoa(last) + `}}`
	return FileThreadRaw(path, position)
}

// wraps one file-feedback thread in an otherwise minimal document.
func DocumentWith(thread string) string {
	return Document([]string{}, []string{thread})
}

// a minimal document holding `discussion` and `fileFeedback`.
func Document(discussion []string, fileFeedback []string) string {
	return `{
		"pull_request": ` + MinimalPullRequest + `,
		"discussion": [` + strings.Join(discussion, ",") + `],
		"file_feedback": [` + strings.Join(fileFeedback, ",") + `]
	}`
}
