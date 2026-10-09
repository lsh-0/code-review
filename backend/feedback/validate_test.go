package feedback

import (
	"testing"
)

// a document built in memory, with no JSON involved: validation needs nothing but
// the value.
func validated(threads []Thread, reported int) Document {
	discussion := make([]Thread, 0)
	fileFeedback := make([]Thread, 0)
	for _, thread := range threads {
		if thread.Location.Kind == LocationPullRequest {
			discussion = append(discussion, thread)
		} else {
			fileFeedback = append(fileFeedback, thread)
		}
	}
	return Document{
		PullRequest: PullRequest{
			ID: 1, Title: "t", State: "open",
			Author:               Author{DisplayName: "A", Kind: AuthorHuman},
			CreatedAt:            "2026-08-04T09:00:00.000000Z",
			ReportedCommentCount: reported,
		},
		Discussion:   discussion,
		FileFeedback: fileFeedback,
	}
}

func liveComment(id int, createdAt string, replies ...Comment) Comment {
	return Comment{
		ID: id, Author: Author{DisplayName: "A", Kind: AuthorHuman},
		Body: Body{Kind: BodyMarkdown, Text: "text"}, CreatedAt: createdAt, Replies: replies,
	}
}

func deletedComment(id int) Comment {
	comment := liveComment(id, "2026-08-04T10:00:00.000000Z")
	comment.Body = Body{Kind: BodyDeleted}
	return comment
}

func discussionThread(root Comment) Thread {
	return Thread{
		Location:   Location{Kind: LocationPullRequest},
		Resolution: Resolution{State: ResolutionUnresolved},
		Root:       root,
	}
}

func codesOf(findings []Finding) map[Code]Finding {
	byCode := make(map[Code]Finding)
	for _, finding := range findings {
		byCode[finding.Code] = finding
	}
	return byCode
}

const at = "2026-08-04T10:00:00.000000Z"

func TestCheckCountsReconcile(t *testing.T) {
	// three live comments and two deleted, so the source would report three.
	given := validated([]Thread{
		discussionThread(liveComment(1, at)),
		discussionThread(liveComment(2, at)),
		discussionThread(liveComment(3, at)),
		discussionThread(deletedComment(4)),
		discussionThread(deletedComment(5)),
	}, 3)

	actual := codesOf(Check(given))

	if _, found := actual[CommentCountMismatch]; found {
		t.Errorf("expected no mismatch, got %v", actual[CommentCountMismatch])
	}
}

func TestCheckCountsDoNotReconcile(t *testing.T) {
	given := validated([]Thread{discussionThread(liveComment(1, at)), discussionThread(liveComment(2, at))}, 166)

	actual, found := codesOf(Check(given))[CommentCountMismatch]

	if !found {
		t.Fatal("expected a count mismatch")
	}
	expected := "2 comments retrieved less 0 without a body is 2, but the pull request reports 166"
	if actual.Detail != expected || actual.Severity != Warning {
		t.Errorf("got %v, want a warning with detail %q", actual, expected)
	}
}

func TestCheckNoCommentsNoMismatch(t *testing.T) {
	given := validated(nil, 0)
	if actual := Check(given); len(actual) != 0 {
		t.Errorf("expected no findings, got %v", actual)
	}
}

func TestCheckDeletedCommentsNotedAtDebug(t *testing.T) {
	given := validated([]Thread{discussionThread(deletedComment(1))}, 0)

	actual, found := codesOf(Check(given))[DeletedCommentsPresent]

	if !found || actual.Severity != Debug {
		t.Errorf("expected a debug note, got %v", actual)
	}
	if actual.Detail != "1 of 1 comments have no body" {
		t.Errorf("detail: got %q", actual.Detail)
	}
}

func TestCheckWellFormedDocumentIsClean(t *testing.T) {
	given := validated([]Thread{
		discussionThread(liveComment(1, at, liveComment(2, "2026-08-04T11:00:00.000000Z"))),
	}, 2)

	if actual := Check(given); len(actual) != 0 {
		t.Errorf("expected no findings, got %v", actual)
	}
}

func TestCheckDuplicateIdentifiers(t *testing.T) {
	given := validated([]Thread{discussionThread(liveComment(1, at)), discussionThread(liveComment(1, at))}, 2)

	actual, found := codesOf(Check(given))[DuplicateCommentID]

	if !found || actual.Severity != Error {
		t.Errorf("expected a duplicate identifier error, got %v", actual)
	}
}

func TestCheckPartitionViolation(t *testing.T) {
	given := validated(nil, 1)
	given.Discussion = []Thread{{
		Location:   Location{Kind: LocationFileRevision, Path: "a.go", Position: Position{Kind: PositionWholeFile}},
		Resolution: Resolution{State: ResolutionUnresolved},
		Root:       liveComment(7, at),
	}}

	actual, found := codesOf(Check(given))[PartitionViolation]

	if !found || actual.Detail != "thread rooted at comment 7 is in the wrong collection" {
		t.Errorf("expected a partition violation naming comment 7, got %v", actual)
	}
}

func TestCheckEmptyAnchorPath(t *testing.T) {
	given := validated([]Thread{{
		Location:   Location{Kind: LocationFileRevision, Position: Position{Kind: PositionWholeFile}},
		Resolution: Resolution{State: ResolutionUnresolved},
		Root:       liveComment(3, at),
	}}, 1)

	if _, found := codesOf(Check(given))[InlineWithoutPath]; !found {
		t.Error("expected an empty anchor path to be reported")
	}
}

// checked over the built forest at every depth, so a tree rebuilt wrongly is caught.
func TestCheckReplyPrecedesParentAtDepth(t *testing.T) {
	given := validated([]Thread{
		discussionThread(liveComment(1, at,
			liveComment(2, "2026-08-04T11:00:00.000000Z",
				liveComment(3, "2026-08-04T10:30:00.000000Z")))),
	}, 3)

	actual, found := codesOf(Check(given))[ReplyPrecedesParent]

	if !found || actual.Detail != "reply 3 was created before its parent 2" {
		t.Errorf("expected reply 3 before parent 2, got %v", actual)
	}
}

func TestCheckResolvedWithoutDetail(t *testing.T) {
	thread := discussionThread(liveComment(4, at))
	thread.Resolution = Resolution{State: ResolutionResolvedWithoutInfo}
	given := validated([]Thread{thread}, 1)

	actual, found := codesOf(Check(given))[ResolutionDetailMissing]

	if !found || actual.Severity != Warning {
		t.Errorf("expected a warning, got %v", actual)
	}
}

func TestCommentsWalksDepthFirstInThreadOrder(t *testing.T) {
	given := validated([]Thread{
		discussionThread(liveComment(1, at, liveComment(2, at, liveComment(3, at)), liveComment(4, at))),
		discussionThread(liveComment(5, at)),
	}, 5)

	expected := []int{1, 2, 3, 4, 5}
	actual := make([]int, 0)
	for _, comment := range Comments(given) {
		actual = append(actual, comment.ID)
	}
	for i := range expected {
		if i >= len(actual) || actual[i] != expected[i] {
			t.Fatalf("got %v, want %v", actual, expected)
		}
	}
}
