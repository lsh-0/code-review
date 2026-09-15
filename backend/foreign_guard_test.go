package main

import (
	"code-review/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// an app holding one imported root comment on `filePath`, persisted, so a guard test
// can assert the state file is untouched by a refused mutation.
func appWithImportedComment(t *testing.T) (*App, string, string) {
	t.Helper()
	tmpDir := t.TempDir()

	app := &App{
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
		dataDir:   tmpDir,
		userName:  "Test User",
		statePath: filepath.Join(tmpDir, "test.json"),
	}

	const filePath = "config/server.conf"
	file := app.review.AddFileDiff(filePath)
	imported := &model.Comment{
		ID:      "imported-1",
		Author:  "review-bot (bot)",
		Content: "**HIGH** — a finding",
		Status:  model.CommentStatusActive,
		Anchors: []model.Anchor{{Blob: "sha", LineNumber: 69, Offset: 1, Context: []string{"a", "b", "c"}}},
		Origin:  &model.Origin{Source: "bb:1430", ForeignID: "856483458"},
	}
	file.Comments = append(file.Comments, imported)

	if err := app.persist(); err != nil {
		t.Fatalf("persist failed: %v", err)
	}
	return app, filePath, imported.ID
}

// the state file's bytes, for an unchanged-by-refusal assertion.
func stateBytes(t *testing.T, app *App) []byte {
	t.Helper()
	data, err := os.ReadFile(app.statePath)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}
	return data
}

func TestUpdateForeignCommentIsRefused(t *testing.T) {
	app, filePath, commentID := appWithImportedComment(t)
	before := stateBytes(t, app)

	_, err := app.UpdateComment(filePath, commentID, "rewritten")

	if err == nil {
		t.Fatal("expected updating an imported comment to be refused")
	}
	if !strings.Contains(err.Error(), "imported") {
		t.Errorf("error %q does not state the comment was imported", err.Error())
	}
	if string(stateBytes(t, app)) != string(before) {
		t.Error("expected a refused update to leave the state file unchanged")
	}
	if app.review.GetFileDiff(filePath).GetComment(commentID).Content == "rewritten" {
		t.Error("expected the comment's content to be unchanged in memory")
	}
}

func TestDeleteForeignCommentIsRefused(t *testing.T) {
	app, filePath, commentID := appWithImportedComment(t)
	before := stateBytes(t, app)

	_, err := app.DeleteComment(filePath, commentID)

	if err == nil {
		t.Fatal("expected deleting an imported comment to be refused")
	}
	if !strings.Contains(err.Error(), "imported") {
		t.Errorf("error %q does not state the comment was imported", err.Error())
	}
	if string(stateBytes(t, app)) != string(before) {
		t.Error("expected a refused delete to leave the state file unchanged")
	}
	if app.review.GetFileDiff(filePath).GetComment(commentID) == nil {
		t.Error("expected the comment to still be present")
	}
}

// resolve, ignore and reactivate are local status changes over an immutable body, so
// each must work on an imported comment.
func TestForeignCommentStatusActionsSucceed(t *testing.T) {
	given := []struct {
		name     string
		act      func(app *App, filePath string, commentID string) (string, error)
		expected model.CommentStatus
	}{
		{"resolve", (*App).ResolveComment, model.CommentStatusResolved},
		{"ignore", (*App).IgnoreComment, model.CommentStatusIgnored},
	}

	for _, test := range given {
		t.Run(test.name, func(t *testing.T) {
			app, filePath, commentID := appWithImportedComment(t)

			if _, err := test.act(app, filePath, commentID); err != nil {
				t.Fatalf("%s failed on an imported comment: %v", test.name, err)
			}

			actual := app.review.GetFileDiff(filePath).GetComment(commentID).Status
			if actual != test.expected {
				t.Errorf("status: got %q, want %q", actual, test.expected)
			}
		})
	}
}

func TestForeignCommentReactivateSucceeds(t *testing.T) {
	app, filePath, commentID := appWithImportedComment(t)

	if _, err := app.ResolveComment(filePath, commentID); err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if _, err := app.ReactivateComment(filePath, commentID); err != nil {
		t.Fatalf("reactivate failed on an imported comment: %v", err)
	}

	actual := app.review.GetFileDiff(filePath).GetComment(commentID).Status
	if actual != model.CommentStatusActive {
		t.Errorf("status: got %q, want %q", actual, model.CommentStatusActive)
	}
}

func TestReplyToForeignCommentSucceedsAndIsLocal(t *testing.T) {
	app, filePath, commentID := appWithImportedComment(t)

	if _, err := app.AddReply(filePath, commentID, "I will fix this"); err != nil {
		t.Fatalf("reply to an imported comment failed: %v", err)
	}

	file := app.review.GetFileDiff(filePath)
	var reply *model.Comment
	for _, comment := range file.Comments {
		if comment.ParentID == commentID {
			reply = comment
		}
	}
	if reply == nil {
		t.Fatal("expected the reply to be attached to the imported comment")
	}
	if reply.IsForeign() {
		t.Error("expected a locally written reply to be local")
	}
	if reply.Author != "Test User" {
		t.Errorf("author: got %q, want %q", reply.Author, "Test User")
	}
}

// a local reply to an imported comment is itself a local comment, so the guards must
// not extend to it.
func TestLocalReplyToForeignCommentIsEditableAndDeletable(t *testing.T) {
	app, filePath, commentID := appWithImportedComment(t)

	if _, err := app.AddReply(filePath, commentID, "first wording"); err != nil {
		t.Fatalf("reply failed: %v", err)
	}

	file := app.review.GetFileDiff(filePath)
	replyID := ""
	for _, comment := range file.Comments {
		if comment.ParentID == commentID {
			replyID = comment.ID
		}
	}

	if _, err := app.UpdateComment(filePath, replyID, "second wording"); err != nil {
		t.Fatalf("expected a local reply to be editable: %v", err)
	}
	if file.GetComment(replyID).Content != "second wording" {
		t.Errorf("content: got %q, want %q", file.GetComment(replyID).Content, "second wording")
	}

	if _, err := app.DeleteComment(filePath, replyID); err != nil {
		t.Fatalf("expected a local reply to be deletable: %v", err)
	}
	if app.review.GetFileDiff(filePath).GetComment(replyID) != nil {
		t.Error("expected the local reply to be removed")
	}
}

// the guards apply to a review-level imported comment too, which resolves through a
// different surface.
func TestReviewLevelForeignCommentIsGuarded(t *testing.T) {
	tmpDir := t.TempDir()
	app := &App{
		review:    model.NewReview("/some/repo/path", "feature", "main"),
		repoPath:  "/some/repo/path",
		dataDir:   tmpDir,
		userName:  "Test User",
		statePath: filepath.Join(tmpDir, "test.json"),
	}
	app.review.Comments = append(app.review.Comments, &model.Comment{
		ID:      "imported-overall",
		Author:  "review-bot (bot)",
		Content: "## Code Review",
		Status:  model.CommentStatusActive,
		Origin:  &model.Origin{Source: "bb:1430", ForeignID: "856483516"},
	})
	if err := app.persist(); err != nil {
		t.Fatalf("persist failed: %v", err)
	}

	if _, err := app.UpdateComment("", "imported-overall", "rewritten"); err == nil {
		t.Error("expected updating an imported review-level comment to be refused")
	}
	if _, err := app.DeleteComment("", "imported-overall"); err == nil {
		t.Error("expected deleting an imported review-level comment to be refused")
	}
	if _, err := app.ResolveComment("", "imported-overall"); err != nil {
		t.Errorf("expected resolving an imported review-level comment to succeed: %v", err)
	}
}
