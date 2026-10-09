package bitbucket

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"code-review/feedback"
)

// {1 Builders for synthetic wire values}

//go:fix inline
func strptr(s string) *string { return new(s) }

//go:fix inline
func intptr(i int) *int { return new(i) }

func human() wireUser {
	return wireUser{DisplayName: "Human One", AccountID: new("acc"), UUID: new("{u1}"), Nickname: new("nick"), Type: "user"}
}

// a live, unanchored comment created at 13:40 by a human. Each test changes what
// it is about.
func wire(id int, edits ...func(*wireComment)) wireComment {
	comment := wireComment{
		ID:        id,
		CreatedOn: "2026-08-04T13:40:17.000000+00:00",
		BodyRaw:   "text",
		Markup:    new("markdown"),
		User:      human(),
	}
	for _, edit := range edits {
		edit(&comment)
	}
	return comment
}

func parent(id int) func(*wireComment) {
	return func(c *wireComment) { c.ParentID = new(id) }
}

func createdOn(text string) func(*wireComment) {
	return func(c *wireComment) { c.CreatedOn = text }
}

func anchored(inline wireInline) func(*wireComment) {
	return func(c *wireComment) { c.Inline = &inline }
}

func wirePR(edits ...func(*wirePullRequest)) wirePullRequest {
	pr := wirePullRequest{
		ID: 1, Title: "t", State: "OPEN", Author: human(),
		CreatedOn:   "2026-08-04T13:00:00.000000+00:00",
		Source:      wireRevision{Branch: "feature", Commit: new("abc")},
		Destination: wireRevision{Branch: "master", Commit: new("def")},
	}
	for _, edit := range edits {
		edit(&pr)
	}
	return pr
}

func normaliseComments(comments ...wireComment) (feedback.Document, []feedback.Finding) {
	return normalise(wirePR(), comments)
}

func hasCode(findings []feedback.Finding, code feedback.Code) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func positionName(position feedback.Position) string {
	spanName := func(s feedback.Span) string {
		if s.First == s.Last {
			return strconv.Itoa(s.First)
		}
		return fmt.Sprintf("%d-%d", s.First, s.Last)
	}
	switch position.Kind {
	case feedback.PositionOriginal, feedback.PositionUpdated:
		return position.Kind + " " + spanName(position.Lines)
	case feedback.PositionUnchanged:
		return "unchanged " + spanName(position.OriginalLines) + "/" + spanName(position.UpdatedLines)
	default:
		return position.Kind
	}
}

func ids(comments []feedback.Comment) []int {
	result := make([]int, 0, len(comments))
	for _, comment := range comments {
		result = append(result, comment.ID)
	}
	return result
}

// {1 Anchor refinement: all sixteen presence combinations}

// `from`, `to`, `start_from`, `start_to`, then the expected position. Only `from`
// and `to` select the case. The start fields widen a span and are ignored when
// their line is absent.
func TestAnchorRefinementTable(t *testing.T) {
	n := func(i int) *int { return &i }
	given := []struct {
		from, to, startFrom, startTo *int
		expected                     string
	}{
		{nil, nil, nil, nil, "whole_file"},
		{nil, nil, n(3), nil, "whole_file"},
		{nil, nil, nil, n(3), "whole_file"},
		{nil, nil, n(3), n(4), "whole_file"},
		{n(40), nil, nil, nil, "original 40"},
		{n(40), nil, n(36), nil, "original 36-40"},
		{n(40), nil, nil, n(9), "original 40"},
		{n(40), nil, n(36), n(9), "original 36-40"},
		{nil, n(12), nil, nil, "updated 12"},
		{nil, n(12), nil, n(8), "updated 8-12"},
		{nil, n(12), n(9), nil, "updated 12"},
		{nil, n(12), n(9), n(8), "updated 8-12"},
		{n(40), n(42), nil, nil, "unchanged 40/42"},
		{n(40), n(42), n(36), nil, "unchanged 36-40/42"},
		{n(40), n(42), nil, n(38), "unchanged 40/38-42"},
		{n(40), n(42), n(36), n(38), "unchanged 36-40/38-42"},
	}
	if len(given) != 16 {
		t.Fatalf("the table covers %d combinations, want 16", len(given))
	}
	for i, test := range given {
		inline := wireInline{Path: "src/a.ml", From: test.from, To: test.to, StartFrom: test.startFrom, StartTo: test.startTo}
		actual, findings := positionOfInline(inline, 1)
		if positionName(actual) != test.expected {
			t.Errorf("row %d: got %q, want %q", i, positionName(actual), test.expected)
		}
		if len(findings) != 0 {
			t.Errorf("row %d: a well-formed anchor produced findings %v", i, findings)
		}
	}
}

func TestInvertedSpanIsClampedAndReported(t *testing.T) {
	given := wireInline{Path: "src/a.ml", To: new(20), StartTo: new(30)}

	actual, findings := positionOfInline(given, 7)

	if positionName(actual) != "updated 20" {
		t.Errorf("got %q, want the end line", positionName(actual))
	}
	if !hasCode(findings, feedback.InlineSpanInverted) {
		t.Error("expected the inversion to be reported")
	}
}

func TestEmptyPathBecomesDiscussion(t *testing.T) {
	actual, findings := normaliseComments(wire(1, anchored(wireInline{Path: "", To: new(5)})))

	if len(actual.FileFeedback) != 0 || len(actual.Discussion) != 1 {
		t.Errorf("got %d file and %d discussion threads, want 0 and 1", len(actual.FileFeedback), len(actual.Discussion))
	}
	if !hasCode(findings, feedback.InlineWithoutPath) {
		t.Error("expected the empty path to be reported")
	}
}

// {1 Partition}

func TestAnchoredAndUnanchoredPartition(t *testing.T) {
	actual, _ := normaliseComments(wire(1), wire(2, anchored(wireInline{Path: "src/a.ml", To: new(12)})))

	if len(actual.Discussion) != 1 || actual.Discussion[0].Location.Kind != feedback.LocationPullRequest {
		t.Errorf("discussion: %+v", actual.Discussion)
	}
	if len(actual.FileFeedback) != 1 || actual.FileFeedback[0].Location.Kind != feedback.LocationFileRevision {
		t.Errorf("file feedback: %+v", actual.FileFeedback)
	}
}

// {1 Forest reconstruction}

func TestSingleLevelReply(t *testing.T) {
	actual, findings := normaliseComments(wire(1), wire(2, parent(1)))

	threads := actual.Threads()
	if len(threads) != 1 || len(threads[0].Root.Replies) != 1 {
		t.Fatalf("got %+v, want one thread with one reply", threads)
	}
	if len(feedback.Comments(actual)) != 2 || len(findings) != 0 {
		t.Errorf("got %d comments and findings %v", len(feedback.Comments(actual)), findings)
	}
}

func TestNestedReplyChain(t *testing.T) {
	actual, _ := normaliseComments(
		wire(1, createdOn("2026-08-04T10:00:00.000000+00:00")),
		wire(2, parent(1), createdOn("2026-08-04T11:00:00.000000+00:00")),
		wire(3, parent(2), createdOn("2026-08-04T12:00:00.000000+00:00")),
	)

	threads := actual.Threads()
	if len(threads) != 1 {
		t.Fatalf("threads: got %d, want 1", len(threads))
	}
	root := threads[0].Root
	if len(root.Replies) != 1 || len(root.Replies[0].Replies) != 1 || root.Replies[0].Replies[0].ID != 3 {
		t.Errorf("expected 1 → 2 → 3, got %+v", root)
	}
}

func TestOrphanReplyIsPromoted(t *testing.T) {
	actual, findings := normaliseComments(wire(2, parent(999)))

	if len(actual.Threads()) != 1 || len(feedback.Comments(actual)) != 1 {
		t.Errorf("expected the orphan as a thread root, got %+v", actual)
	}
	if !hasCode(findings, feedback.OrphanReply) {
		t.Error("expected the orphan to be reported")
	}
}

// two comments naming each other: neither is a root, so without the sweep both
// would vanish.
func TestReplyCycleTerminates(t *testing.T) {
	actual, findings := normaliseComments(wire(1, parent(2)), wire(2, parent(1)))

	if !hasCode(findings, feedback.ReplyCycle) {
		t.Error("expected the cycle to be reported")
	}
	got := ids(feedback.Comments(actual))
	sort.Ints(got)
	if !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("got comments %v, want 1 and 2 exactly once", got)
	}
}

func TestDuplicateIdentifierIsReported(t *testing.T) {
	actual, findings := normaliseComments(wire(1), wire(1, func(c *wireComment) { c.BodyRaw = "other" }))

	if !hasCode(findings, feedback.DuplicateCommentID) {
		t.Error("expected the duplicate to be reported")
	}
	comments := feedback.Comments(actual)
	if len(comments) != 1 || comments[0].Body.Text != "text" {
		t.Errorf("expected the first occurrence only, got %+v", comments)
	}
}

func TestDeletedCommentRetainsReplies(t *testing.T) {
	actual, _ := normaliseComments(
		wire(1, func(c *wireComment) { c.Deleted = true; c.BodyRaw = "" }),
		wire(2, parent(1)),
		wire(3, parent(1)),
	)

	root := actual.Threads()[0].Root
	if root.Body.Kind != feedback.BodyDeleted || root.ID != 1 || len(root.Replies) != 2 {
		t.Errorf("expected deleted root 1 with two replies, got %+v", root)
	}
}

func TestBodyCases(t *testing.T) {
	given := []struct {
		name     string
		deleted  bool
		raw      string
		expected string
		code     feedback.Code
	}{
		{"live", false, "text", feedback.BodyMarkdown, ""},
		{"deleted, empty", true, "", feedback.BodyDeleted, ""},
		{"deleted with text", true, "left over", feedback.BodyDeleted, feedback.DeletedWithBody},
		{"live, empty", false, "", feedback.BodyDeleted, feedback.LiveCommentEmptyBody},
		{"live, blank", false, " \t\n", feedback.BodyDeleted, feedback.LiveCommentEmptyBody},
	}
	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			actual, findings := body(1, test.deleted, test.raw)
			if actual.Kind != test.expected {
				t.Errorf("kind: got %q, want %q", actual.Kind, test.expected)
			}
			if test.code == "" && len(findings) != 0 {
				t.Errorf("unexpected findings %v", findings)
			}
			if test.code != "" && !hasCode(findings, test.code) {
				t.Errorf("expected %s, got %v", test.code, findings)
			}
		})
	}
}

// {1 Ordering}

func TestRepliesOrderedByCreationTime(t *testing.T) {
	actual, _ := normaliseComments(
		wire(1, createdOn("2026-08-04T10:00:00.000000+00:00")),
		wire(4, parent(1), createdOn("2026-08-04T13:00:00.000000+00:00")),
		wire(2, parent(1), createdOn("2026-08-04T11:00:00.000000+00:00")),
		wire(3, parent(1), createdOn("2026-08-04T12:00:00.000000+00:00")),
	)

	expected := []int{2, 3, 4}
	if got := ids(actual.Threads()[0].Root.Replies); !reflect.DeepEqual(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestEqualTimestampsBrokenByIdentifier(t *testing.T) {
	same := "2026-08-04T11:00:00.000000+00:00"
	actual, _ := normaliseComments(
		wire(1, createdOn("2026-08-04T10:00:00.000000+00:00")),
		wire(9, parent(1), createdOn(same)),
		wire(5, parent(1), createdOn(same)),
	)

	expected := []int{5, 9}
	if got := ids(actual.Threads()[0].Root.Replies); !reflect.DeepEqual(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

// a timestamp is compared as a time, not as text: an offset changes the instant.
func TestOrderingUsesTheInstantNotTheText(t *testing.T) {
	actual, _ := normaliseComments(
		wire(1, createdOn("2026-08-04T10:00:00+00:00")),
		wire(2, parent(1), createdOn("2026-08-04T12:00:00+05:00")), // 07:00 UTC
		wire(3, parent(1), createdOn("2026-08-04T11:00:00+00:00")),
	)

	expected := []int{2, 3}
	if got := ids(actual.Threads()[0].Root.Replies); !reflect.DeepEqual(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
	if actual.Threads()[0].Root.Replies[0].CreatedAt != "2026-08-04T07:00:00.000000Z" {
		t.Errorf("expected the timestamp rendered in UTC, got %q", actual.Threads()[0].Root.Replies[0].CreatedAt)
	}
}

// {1 Author kind and resolution}

func TestAuthorKindMapping(t *testing.T) {
	bot := wireUser{DisplayName: "bot", Type: "app_user", Kind: new("workspace_access_token")}
	actual, findings := normaliseComments(wire(1), wire(2, func(c *wireComment) { c.User = bot }))

	byID := map[int]feedback.Author{}
	for _, comment := range feedback.Comments(actual) {
		byID[comment.ID] = comment.Author
	}
	if byID[1].Kind != feedback.AuthorHuman {
		t.Errorf("comment 1: got %q, want human", byID[1].Kind)
	}
	if byID[2].Kind != feedback.AuthorApplication || byID[2].ApplicationKind != "workspace_access_token" {
		t.Errorf("comment 2: got %+v, want an application", byID[2])
	}
	if len(findings) != 0 {
		t.Errorf("unexpected findings %v", findings)
	}
}

func TestUnknownAuthorTypeIsItsOwnCase(t *testing.T) {
	martian := human()
	martian.Type = "martian"
	actual, findings := normaliseComments(wire(1, func(c *wireComment) { c.User = martian }))

	author := feedback.Comments(actual)[0].Author
	if author.Kind != feedback.AuthorUnknown || author.ReportedType != "martian" {
		t.Errorf("expected an unknown author carrying its type, got %+v", author)
	}
	if !hasCode(findings, feedback.UnknownAuthorType) {
		t.Error("expected the unknown type to be reported")
	}
}

// the import schema requires a name, so a nameless author gets a placeholder.
func TestAuthorWithoutANameIsGivenOne(t *testing.T) {
	nameless := human()
	nameless.DisplayName = " "
	actual, findings := normaliseComments(wire(1, func(c *wireComment) { c.User = nameless }))

	if name := feedback.Comments(actual)[0].Author.DisplayName; name != unnamedAuthor {
		t.Errorf("got %q, want %q", name, unnamedAuthor)
	}
	if !hasCode(findings, feedback.AuthorWithoutName) {
		t.Error("expected the missing name to be reported")
	}
}

func TestResolutionStates(t *testing.T) {
	actual, findings := normaliseComments(
		wire(1),
		wire(2, func(c *wireComment) { c.Resolution = wireResolution{State: resolutionEmpty} }),
		wire(3, func(c *wireComment) {
			c.Resolution = wireResolution{State: resolutionDetailed, CreatedOn: "2026-08-05T09:00:00.000000+00:00", User: human()}
		}),
	)

	byID := map[int]feedback.Resolution{}
	for _, thread := range actual.Threads() {
		byID[thread.Root.ID] = thread.Resolution
	}
	if byID[1].State != feedback.ResolutionUnresolved {
		t.Errorf("absent: got %q", byID[1].State)
	}
	if byID[2].State != feedback.ResolutionResolvedWithoutInfo {
		t.Errorf("{}: got %q", byID[2].State)
	}
	if byID[3].State != feedback.ResolutionResolved || byID[3].By == nil || byID[3].At != "2026-08-05T09:00:00.000000Z" {
		t.Errorf("expanded: got %+v", byID[3])
	}
	if !hasCode(findings, feedback.ResolutionDetailMissing) {
		t.Error("expected the detail-less resolution to be reported")
	}
}

func TestResolutionOnAReplyIsReported(t *testing.T) {
	_, findings := normaliseComments(wire(1), wire(2, parent(1), func(c *wireComment) {
		c.Resolution = wireResolution{State: resolutionEmpty}
	}))

	if !hasCode(findings, feedback.ResolutionOnReply) {
		t.Error("expected the reply's resolution to be reported")
	}
}

func TestPullRequestStates(t *testing.T) {
	given := []struct {
		wire     string
		expected string
		reported bool
	}{
		{"OPEN", "open", false},
		{"MERGED", "merged", false},
		{"DECLINED", "declined", false},
		{"SUPERSEDED", "superseded", false},
		{"ARCHIVED", "ARCHIVED", true},
	}
	for _, test := range given {
		actual, findings := normalise(wirePR(func(pr *wirePullRequest) { pr.State = test.wire }), nil)
		if actual.PullRequest.State != test.expected {
			t.Errorf("%s: got %q, want %q", test.wire, actual.PullRequest.State, test.expected)
		}
		if hasCode(findings, feedback.UnknownPullRequestState) != test.reported {
			t.Errorf("%s: reported %v, want %v", test.wire, !test.reported, test.reported)
		}
	}
}

func TestUnparseableTimestampIsReported(t *testing.T) {
	actual, findings := normaliseComments(wire(1, createdOn("not a timestamp")))

	if !hasCode(findings, feedback.UnparseableTimestamp) {
		t.Error("expected the timestamp to be reported")
	}
	comments := feedback.Comments(actual)
	if len(comments) != 1 || comments[0].CreatedAt != "1970-01-01T00:00:00.000000Z" {
		t.Errorf("expected the comment present at the epoch, got %+v", comments)
	}
}

func TestReplyPrecedingItsParentIsReported(t *testing.T) {
	_, findings := normaliseComments(
		wire(1, createdOn("2026-08-04T12:00:00.000000+00:00")),
		wire(2, parent(1), createdOn("2026-08-04T10:00:00.000000+00:00")),
	)

	if !hasCode(findings, feedback.ReplyPrecedesParent) {
		t.Error("expected the chronology to be reported")
	}
}

func TestReplyAnchorConflictIsReported(t *testing.T) {
	_, findings := normaliseComments(
		wire(1, anchored(wireInline{Path: "a.go", To: new(1)})),
		wire(2, parent(1), anchored(wireInline{Path: "b.go", To: new(1)})),
	)

	if !hasCode(findings, feedback.ReplyAnchorConflict) {
		t.Error("expected the conflicting anchor to be reported")
	}
}

func TestEmptyDescriptionBecomesNull(t *testing.T) {
	actual, _ := normalise(wirePR(func(pr *wirePullRequest) { pr.Description = "  " }), nil)
	if actual.PullRequest.Description != nil {
		t.Errorf("got %q, want null", *actual.PullRequest.Description)
	}
}

// an anchor naming line 0 would produce a span the import schema rejects, failing
// the whole fetch. Its thread is kept on its file as whole-file feedback.
func TestLineBelowOneBecomesWholeFile(t *testing.T) {
	given := []wireInline{
		{Path: "a.go", To: new(0)},
		{Path: "a.go", From: new(-3)},
		{Path: "a.go", To: new(5), StartTo: new(0)},
	}
	for _, inline := range given {
		actual, findings := positionOfInline(inline, 1)
		if actual.Kind != feedback.PositionWholeFile {
			t.Errorf("%+v: got %q, want whole_file", inline, positionName(actual))
		}
		if !hasCode(findings, feedback.InlineLineOutOfRange) {
			t.Errorf("%+v: expected the line to be reported", inline)
		}
	}
}
