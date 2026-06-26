package schema

import (
	"strings"
	"testing"

	"code-review/model"
)

// a fully-populated review that conforms to the schema: review metadata, a
// file diff with an anchored comment and a reply, a review-level comment, and a
// marked file. Tests mutate a copy of this to produce invalid variants.
func validReview() *model.Review {
	return &model.Review{
		Version:      Version,
		ID:           "rev1",
		RepoPath:     "/repo",
		SourceBranch: "feature",
		TargetBranch: "main",
		Files: []*model.FileDiff{
			{
				FilePath: "main.go",
				Comments: []*model.Comment{
					{
						ID:      "c1",
						Author:  "alice",
						Content: "note",
						Status:  model.CommentStatusActive,
						Anchors: []model.Anchor{
							{Blob: "abc", LineNumber: 10, Offset: 1, Context: []string{"a", "b", "c"}},
						},
					},
					{ID: "c2", ParentID: "c1", Author: "bob", Content: "reply", Status: model.CommentStatusActive},
				},
			},
		},
		Comments: []*model.Comment{
			{ID: "r1", Author: "alice", Content: "overall", Status: model.CommentStatusResolved},
		},
		MarkedFiles: model.MarkedFiles{{Path: "other.go", Blob: "def"}},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*model.Review)
		wantErr bool
	}{
		{
			name:   "conforming current file",
			mutate: func(*model.Review) {},
		},
		{
			name:   "conforming unversioned file",
			mutate: func(r *model.Review) { r.Version = "" },
		},
		{
			name:    "missing comment id",
			mutate:  func(r *model.Review) { r.Files[0].Comments[0].ID = "" },
			wantErr: true,
		},
		{
			name:    "missing review id",
			mutate:  func(r *model.Review) { r.ID = "" },
			wantErr: true,
		},
		{
			name:    "bad comment status",
			mutate:  func(r *model.Review) { r.Files[0].Comments[0].Status = "bogus" },
			wantErr: true,
		},
		{
			name:    "version not matching schema",
			mutate:  func(r *model.Review) { r.Version = "9.9.9" },
			wantErr: true,
		},
		{
			name:    "empty marked-file path",
			mutate:  func(r *model.Review) { r.MarkedFiles[0].Path = "" },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			review := validReview()
			tt.mutate(review)

			err := Validate(review)
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

// a failed validation should name the schema version so the error is actionable.
func TestValidateErrorMentionsVersion(t *testing.T) {
	review := validReview()
	review.Files[0].Comments[0].Status = "bogus"

	err := Validate(review)
	if err == nil {
		t.Fatal("Validate() = nil, want error")
	}
	if !strings.Contains(err.Error(), Version) {
		t.Errorf("error %q does not mention schema version %q", err, Version)
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		version string
		want    Class
	}{
		{"", ClassUnversioned},
		{Version, ClassCurrent},
		{"9.9.9", ClassMismatched},
		{"0.9.0", ClassMismatched},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := Classify(tt.version); got != tt.want {
				t.Errorf("Classify(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

// the version must come from the embedded schema, not be hard-coded; assert it
// is the expected SchemaVer baseline so a drift in statefile.cue is caught.
func TestVersionIsSchemaVerBaseline(t *testing.T) {
	if Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", Version)
	}
}
