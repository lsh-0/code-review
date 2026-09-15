package main

import (
	"code-review/model"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// run one git command in `dir`, isolated from the developer's own git config.
func gitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// a repository with a committed change on `feature` touching `config/server.conf`,
// so the fixture's feedback on that file has real diff lines to place against.
func initRepoWithServerConfChange(t *testing.T) string {
	t.Helper()
	dir := initGitRepo(t)

	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	lines := make([]string, 0, 100)
	for n := 1; n <= 100; n++ {
		lines = append(lines, "directive_"+itoaCLI(n)+" on;")
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "config", "server.conf"), []byte(body), 0o644); err != nil {
		t.Fatalf("write server.conf: %v", err)
	}

	gitInDir(t, dir, "add", ".")
	gitInDir(t, dir, "commit", "-q", "-m", "add server config")
	return dir
}

func itoaCLI(n int) string {
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	if digits == "" {
		return "0"
	}
	return digits
}

// write the committed fixture document into `dir`, returning its path.
func writeFixtureDocument(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(foreignFixture)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	path := filepath.Join(dir, "1430.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to write document: %v", err)
	}
	return path
}

// the review state file the CLI resolved for `dir`, or "" when none exists.
func resolvedStatePath(t *testing.T, dir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "xdg", "code-review", "*.json"))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

func TestImportCommandMergesAndReports(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)

	if code, _, errOut := runCLIInDir(t, dir, "start"); code != 0 {
		t.Fatalf("start failed: %s", errOut)
	}

	code, out, errOut := runCLIInDir(t, dir, "import", document)
	if code != 0 {
		t.Fatalf("import exit code = %d, stderr: %s", code, errOut)
	}

	if !strings.Contains(out, "imported 7 comments") {
		t.Errorf("report does not state the count: %s", out)
	}
	if !strings.Contains(out, "bb:1430") {
		t.Errorf("report does not name the source: %s", out)
	}
	if !strings.Contains(out, "not in the current diff") {
		t.Errorf("report does not name the unmatched paths: %s", out)
	}

	review, err := LoadReview(resolvedStatePath(t, dir))
	if err != nil {
		t.Fatalf("failed to load the state file: %v", err)
	}
	if len(review.Sources) != 1 {
		t.Errorf("sources: got %d, want 1", len(review.Sources))
	}
	if len(review.GetAllComments()) != 7 {
		t.Errorf("comments: got %d, want 7", len(review.GetAllComments()))
	}
}

// the fixture's comment on config/server.conf sits at line 69, which the repository's
// committed change carries, so it must be placed rather than untethered.
func TestImportPlacesCommentOnAMatchedFile(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")

	if code, _, errOut := runCLIInDir(t, dir, "import", document); code != 0 {
		t.Fatalf("import failed: %s", errOut)
	}

	review, err := LoadReview(resolvedStatePath(t, dir))
	if err != nil {
		t.Fatalf("failed to load the state file: %v", err)
	}

	file := review.GetFileDiff("config/server.conf")
	if file == nil {
		t.Fatal("expected the matched file to carry comments")
	}
	if len(file.Comments) != 1 {
		t.Fatalf("comments on server.conf: got %d, want 1", len(file.Comments))
	}

	comment := file.Comments[0]
	if comment.IsOutdated() {
		t.Error("expected a comment on a present line to be placed, not untethered")
	}
	if comment.CurrentLineNumber() != 69 {
		t.Errorf("line: got %d, want 69", comment.CurrentLineNumber())
	}
	if comment.Anchors[0].Blob == "" {
		t.Error("expected the anchor to carry the file's current blob SHA")
	}
	if len(comment.Anchors[0].Context) == 0 {
		t.Error("expected the anchor to carry a captured context window")
	}
}

func TestImportMissingFileErrorsAndLeavesStateUnchanged(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	runCLIInDir(t, dir, "start")

	statePath := resolvedStatePath(t, dir)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read the state file: %v", err)
	}

	code, _, errOut := runCLIInDir(t, dir, "import", filepath.Join(dir, "absent.json"))

	if code == 0 {
		t.Error("expected a missing document to fail")
	}
	if !strings.Contains(errOut, "absent.json") {
		t.Errorf("error does not name the path: %s", errOut)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to re-read the state file: %v", err)
	}
	if string(after) != string(before) {
		t.Error("expected a failed import to leave the state file unchanged")
	}
}

func TestImportMalformedDocumentErrorsAndLeavesStateUnchanged(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	runCLIInDir(t, dir, "start")

	statePath := resolvedStatePath(t, dir)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read the state file: %v", err)
	}

	given := []struct {
		name    string
		content string
	}{
		{"invalid json", `{"pull_request":`},
		{"unrecognised tag", documentWith(fileThreadRaw("a.go", `{"kind": "sideways"}`))},
		{"no pull request", `{"pull_request": {"title": "x"}, "discussion": [], "file_feedback": []}`},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, "bad.json")
			if err := os.WriteFile(path, []byte(test.content), 0o644); err != nil {
				t.Fatalf("write failed: %v", err)
			}

			code, _, errOut := runCLIInDir(t, dir, "import", path)
			if code == 0 {
				t.Errorf("expected a malformed document to fail, stderr: %s", errOut)
			}

			after, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatalf("failed to re-read the state file: %v", err)
			}
			if string(after) != string(before) {
				t.Error("expected a failed import to leave the state file unchanged")
			}
		})
	}
}

func TestImportWrongArgumentCountPrintsUsage(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	runCLIInDir(t, dir, "start")

	given := [][]string{
		{"import"},
		{"import", "one.json", "two.json"},
	}

	for _, args := range given {
		code, _, errOut := runCLIInDir(t, dir, args...)
		if code == 0 {
			t.Errorf("%v: expected a non-zero exit", args)
		}
		if !strings.Contains(errOut, "usage: code-review import") {
			t.Errorf("%v: error does not print usage: %s", args, errOut)
		}
	}
}

// the document is read, never written: an import must leave it byte-identical.
func TestImportDoesNotModifyTheDocument(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")

	before, err := os.ReadFile(document)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if code, _, errOut := runCLIInDir(t, dir, "import", document); code != 0 {
		t.Fatalf("import failed: %s", errOut)
	}

	after, err := os.ReadFile(document)
	if err != nil {
		t.Fatalf("re-read failed: %v", err)
	}
	if string(after) != string(before) {
		t.Error("expected the imported document to be unchanged")
	}
}

func TestImportedCommentsAppearInList(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	code, out, errOut := runCLIInDir(t, dir, "list")
	if code != 0 {
		t.Fatalf("list failed: %s", errOut)
	}

	var actual []commentView
	if err := json.Unmarshal([]byte(out), &actual); err != nil {
		t.Fatalf("list output is not valid JSON: %v\n%s", err, out)
	}
	if len(actual) != 7 {
		t.Fatalf("listed comments: got %d, want 7", len(actual))
	}
	for _, view := range actual {
		if view.Origin != "bb:1430" {
			t.Errorf("comment %s: origin %q, want %q", view.ID, view.Origin, "bb:1430")
		}
	}
}

// a local comment must report no origin, so an agent can tell which comments it may
// edit from the same output.
func TestListDistinguishesLocalFromImported(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	if code, _, errOut := runCLIInDir(t, dir, "comment", "my own note"); code != 0 {
		t.Fatalf("comment failed: %s", errOut)
	}

	_, out, _ := runCLIInDir(t, dir, "list")
	var actual []commentView
	if err := json.Unmarshal([]byte(out), &actual); err != nil {
		t.Fatalf("list output is not valid JSON: %v", err)
	}

	local := 0
	imported := 0
	for _, view := range actual {
		if view.Origin == "" {
			local++
		} else {
			imported++
		}
	}
	if local != 1 {
		t.Errorf("local comments: got %d, want 1", local)
	}
	if imported != 7 {
		t.Errorf("imported comments: got %d, want 7", imported)
	}
}

func TestStatusCountsImportedComments(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	code, out, errOut := runCLIInDir(t, dir, "status")
	if code != 0 {
		t.Fatalf("status failed: %s", errOut)
	}

	var actual statusView
	if err := json.Unmarshal([]byte(out), &actual); err != nil {
		t.Fatalf("status output is not valid JSON: %v", err)
	}
	if actual.Active != 7 {
		t.Errorf("active: got %d, want 7", actual.Active)
	}
}

func TestResolveAndReplyWorkOnAnImportedComment(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	_, out, _ := runCLIInDir(t, dir, "list")
	var listed []commentView
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatalf("list output is not valid JSON: %v", err)
	}
	target := listed[0].ID

	if code, _, errOut := runCLIInDir(t, dir, "reply", target, "I will fix this"); code != 0 {
		t.Fatalf("reply to an imported comment failed: %s", errOut)
	}
	if code, _, errOut := runCLIInDir(t, dir, "resolve", target); code != 0 {
		t.Fatalf("resolve of an imported comment failed: %s", errOut)
	}

	code, showOut, errOut := runCLIInDir(t, dir, "show", target)
	if code != 0 {
		t.Fatalf("show failed: %s", errOut)
	}
	var shown commentView
	if err := json.Unmarshal([]byte(showOut), &shown); err != nil {
		t.Fatalf("show output is not valid JSON: %v", err)
	}

	if shown.Status != string(model.CommentStatusResolved) {
		t.Errorf("status: got %q, want resolved", shown.Status)
	}
	if len(shown.Replies) != 1 {
		t.Fatalf("replies: got %d, want 1", len(shown.Replies))
	}
	if shown.Replies[0].Origin != "" {
		t.Errorf("expected a local reply to report no origin, got %q", shown.Replies[0].Origin)
	}
	if shown.Origin != "bb:1430" {
		t.Errorf("expected the root to keep its origin, got %q", shown.Origin)
	}
}

// the local reply and the local resolution must both survive a re-import, which is
// the whole point of matching on the foreign identifier.
func TestReimportPreservesLocalReplyAndResolution(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	_, out, _ := runCLIInDir(t, dir, "list")
	var listed []commentView
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatalf("list output is not valid JSON: %v", err)
	}
	target := listed[0].ID

	runCLIInDir(t, dir, "reply", target, "I will fix this")
	runCLIInDir(t, dir, "resolve", target)

	if code, _, errOut := runCLIInDir(t, dir, "import", document); code != 0 {
		t.Fatalf("re-import failed: %s", errOut)
	}

	code, showOut, errOut := runCLIInDir(t, dir, "show", target)
	if code != 0 {
		t.Fatalf("show after re-import failed: %s", errOut)
	}
	var shown commentView
	if err := json.Unmarshal([]byte(showOut), &shown); err != nil {
		t.Fatalf("show output is not valid JSON: %v", err)
	}

	if shown.Status != string(model.CommentStatusResolved) {
		t.Errorf("status after re-import: got %q, want resolved", shown.Status)
	}
	if len(shown.Replies) != 1 {
		t.Errorf("replies after re-import: got %d, want 1", len(shown.Replies))
	}

	review, err := LoadReview(resolvedStatePath(t, dir))
	if err != nil {
		t.Fatalf("failed to load the state file: %v", err)
	}
	// seven imported plus the one local reply.
	if len(review.GetAllComments()) != 8 {
		t.Errorf("comments after re-import: got %d, want 8", len(review.GetAllComments()))
	}
}

func TestImportAppearsInUsage(t *testing.T) {
	code, out, _ := runCLIInDir(t, t.TempDir(), "--help")
	if code != 0 {
		t.Fatalf("help exit code = %d", code)
	}
	if !strings.Contains(out, "import") {
		t.Errorf("usage does not list the import command: %s", out)
	}
}

// the CLI exposes no command that edits or destroys a comment, imported or local, so
// imported feedback keeps the text it was written with and cannot be removed from the
// wrong side. Immutability holds here by the command set, not by a guard, so the
// command set is what this asserts.
func TestNoCommandEditsOrDeletesAComment(t *testing.T) {
	forbidden := []string{"edit", "update", "delete", "remove", "rm", "ignore"}
	for _, name := range forbidden {
		if _, present := commands[name]; present {
			t.Errorf("the CLI exposes %q, which is outside the agent policy", name)
		}
	}
}

// the whole command set, pinned. A command added without a spec change fails here,
// which is the check that keeps the mutation surface conservative as the CLI grows.
func TestCommandSetIsPinned(t *testing.T) {
	expected := []string{
		"comment", "import", "instructions", "list", "reactivate",
		"reply", "resolve", "show", "start", "status", "unmark",
	}
	actual := commandNames()
	if !equalStringSlices(actual, expected) {
		t.Errorf("command set: got %v, want %v", actual, expected)
	}
}

// the import reads a file and nothing else. Asserted structurally, over the source of
// the two files that make up the import path: a future `net/http` or credential read
// added there fails here rather than reaching a user's machine.
func TestImportPathHasNoNetworkOrCredentialAccess(t *testing.T) {
	forbidden := []string{
		"net/http", "\"net\"", "http.Get", "http.Post", "http.Client",
		"os.Getenv", "netrc", "Authorization",
	}
	for _, name := range []string{"foreign_format.go", "foreign_import.go"} {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, token := range forbidden {
			if strings.Contains(string(source), token) {
				t.Errorf("%s references %q: the import must not reach the network or read a credential", name, token)
			}
		}
	}
}

// `import` is the one command that shells out to git, so it is the one that most needs
// the repository-read-only guarantee checked.
func TestImportLeavesTheRepositoryUnchanged(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")

	statusBefore := gitOutput(t, dir, "status", "--porcelain")
	headBefore := gitOutput(t, dir, "rev-parse", "HEAD")
	branchBefore := gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD")

	if code, _, errOut := runCLIInDir(t, dir, "import", document); code != 0 {
		t.Fatalf("import failed: %s", errOut)
	}

	if actual := gitOutput(t, dir, "status", "--porcelain"); actual != statusBefore {
		t.Errorf("working tree changed: got %q, want %q", actual, statusBefore)
	}
	if actual := gitOutput(t, dir, "rev-parse", "HEAD"); actual != headBefore {
		t.Errorf("HEAD moved: got %q, want %q", actual, headBefore)
	}
	if actual := gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); actual != branchBefore {
		t.Errorf("branch changed: got %q, want %q", actual, branchBefore)
	}
}

// the state file is the only thing an import writes under the data directory.
func TestImportWritesOnlyTheStateFile(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")

	dataDir := filepath.Join(dir, "xdg", "code-review")
	before, err := filepath.Glob(filepath.Join(dataDir, "*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	if code, _, errOut := runCLIInDir(t, dir, "import", document); code != 0 {
		t.Fatalf("import failed: %s", errOut)
	}

	after, err := filepath.Glob(filepath.Join(dataDir, "*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !equalStringSlices(after, before) {
		t.Errorf("the data directory gained or lost files: got %v, want %v", after, before)
	}
	if len(after) != 1 {
		t.Errorf("expected exactly one state file, got %d: %v", len(after), after)
	}
}

// the foreign identifier is an internal matching key. It must not appear in any CLI
// output, so an agent never comes to depend on it.
func TestForeignIdentifierIsNotExposed(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	_, listOut, _ := runCLIInDir(t, dir, "list")

	// a real foreign id from the fixture, and the field name it would arrive under.
	for _, forbidden := range []string{"foreign_id", "856483458", "856483516"} {
		if strings.Contains(listOut, forbidden) {
			t.Errorf("list output exposes %q: %s", forbidden, listOut)
		}
	}

	var listed []commentView
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("list output is not valid JSON: %v", err)
	}
	_, showOut, _ := runCLIInDir(t, dir, "show", listed[0].ID)
	for _, forbidden := range []string{"foreign_id", "856483458", "856483516"} {
		if strings.Contains(showOut, forbidden) {
			t.Errorf("show output exposes %q: %s", forbidden, showOut)
		}
	}
}

// hiding a source is a GUI view-state concern and must never reach the state file, so
// an agent sees every comment regardless of what the reviewer has hidden.
func TestStateFileCarriesNoVisibilityState(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	raw, err := os.ReadFile(resolvedStatePath(t, dir))
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	for _, forbidden := range []string{"hidden", "hiddenSources", "visible", "visibility"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("state file carries %q: visibility must stay view state", forbidden)
		}
	}
}

// run a git command in `dir` and return its trimmed output, for invariance checks.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// the report's counts must all reach stdout, not only the imported total. The
// fixture places two comments and leaves four untethered against this repository.
func TestImportReportStatesEveryCount(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")

	code, out, errOut := runCLIInDir(t, dir, "import", document)
	if code != 0 {
		t.Fatalf("import failed: %s", errOut)
	}

	for _, expected := range []string{"7 comments", "1 placed", "5 untethered", "0 replies"} {
		if !strings.Contains(out, expected) {
			t.Errorf("report does not state %q: %s", expected, out)
		}
	}
}

// `reactivate` is in the requirement's list of actions that work on an imported
// comment, and is the one the CLI tests did not exercise.
func TestReactivateWorksOnAnImportedComment(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	_, out, _ := runCLIInDir(t, dir, "list")
	var listed []commentView
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatalf("list output is not valid JSON: %v", err)
	}
	target := listed[0].ID

	if code, _, errOut := runCLIInDir(t, dir, "resolve", target); code != 0 {
		t.Fatalf("resolve failed: %s", errOut)
	}
	if code, _, errOut := runCLIInDir(t, dir, "reactivate", target); code != 0 {
		t.Fatalf("reactivate of an imported comment failed: %s", errOut)
	}

	_, showOut, _ := runCLIInDir(t, dir, "show", target)
	var shown commentView
	if err := json.Unmarshal([]byte(showOut), &shown); err != nil {
		t.Fatalf("show output is not valid JSON: %v", err)
	}
	if shown.Status != string(model.CommentStatusActive) {
		t.Errorf("status: got %q, want active", shown.Status)
	}
}

// `status` counts imported comments in every bucket, not only the active one.
func TestStatusCountsImportedCommentsInEveryBucket(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	_, out, _ := runCLIInDir(t, dir, "list")
	var listed []commentView
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatalf("list output is not valid JSON: %v", err)
	}
	runCLIInDir(t, dir, "resolve", listed[0].ID)

	_, statusOut, _ := runCLIInDir(t, dir, "status")
	var actual statusView
	if err := json.Unmarshal([]byte(statusOut), &actual); err != nil {
		t.Fatalf("status output is not valid JSON: %v", err)
	}

	if actual.Resolved != 1 {
		t.Errorf("resolved: got %d, want 1", actual.Resolved)
	}
	if actual.Active != 6 {
		t.Errorf("active: got %d, want 6", actual.Active)
	}
}

// a review-level imported comment reports an empty file path, which is how the agent
// tells overall feedback from a file-anchored comment.
func TestListReportsEmptyFileForAReviewLevelImportedComment(t *testing.T) {
	dir := initRepoWithServerConfChange(t)
	document := writeFixtureDocument(t, dir)
	runCLIInDir(t, dir, "start")
	runCLIInDir(t, dir, "import", document)

	_, out, _ := runCLIInDir(t, dir, "list")
	var listed []commentView
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatalf("list output is not valid JSON: %v", err)
	}

	reviewLevel := 0
	for _, view := range listed {
		if view.File != "" {
			continue
		}
		reviewLevel++
		if view.Author == "" {
			t.Error("expected a review-level comment to carry its author")
		}
		if view.Content == "" {
			t.Error("expected a review-level comment to carry its content")
		}
		if view.Line != 0 {
			t.Errorf("expected a review-level comment to report line 0, got %d", view.Line)
		}
	}
	if reviewLevel != 1 {
		t.Errorf("review-level comments: got %d, want 1", reviewLevel)
	}
}
