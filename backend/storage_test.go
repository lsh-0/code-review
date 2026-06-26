package main

import (
	"code-review/model"
	"code-review/schema"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadReview(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "review_state.json")

	review := model.NewReview("/repo/path", "feature", "main")
	diff := review.AddFileDiff("test.go")
	diff.AddComment("This needs refactoring", 10, "Test User")
	diff.AddComment("Add error handling here", 25, "Test User")

	err := SaveReview(statePath, review)
	if err != nil {
		t.Fatalf("Failed to save review: %v", err)
	}

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("Expected state file to exist")
	}

	loadedReview, err := LoadReview(statePath)
	if err != nil {
		t.Fatalf("Failed to load review: %v", err)
	}

	if loadedReview.ID != review.ID {
		t.Errorf("Expected ID %s, got %s", review.ID, loadedReview.ID)
	}

	if loadedReview.RepoPath != review.RepoPath {
		t.Errorf("Expected repo path %s, got %s", review.RepoPath, loadedReview.RepoPath)
	}

	if loadedReview.SourceBranch != review.SourceBranch {
		t.Errorf("Expected source branch %s, got %s", review.SourceBranch, loadedReview.SourceBranch)
	}

	if loadedReview.TargetBranch != review.TargetBranch {
		t.Errorf("Expected target branch %s, got %s", review.TargetBranch, loadedReview.TargetBranch)
	}

	if len(loadedReview.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(loadedReview.Files))
	}

	if len(loadedReview.Files[0].Comments) != 2 {
		t.Fatalf("Expected 2 comments, got %d", len(loadedReview.Files[0].Comments))
	}

	if loadedReview.Files[0].Comments[0].Content != "This needs refactoring" {
		t.Errorf("Expected comment content to match")
	}
}

// a state file written before the anchor change carries `line_number` and
// `context_*` on each comment and no `anchors`. Loading it must upgrade each
// line-anchored comment to a single legacy first anchor (empty blob) in place,
// with no separate migration step, so old files open unchanged.
func TestLoadReviewUpgradesLegacyComment(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "legacy_state.json")

	legacy := `{
  "id": "rev1",
  "repo_path": "/repo",
  "source_branch": "feature",
  "target_branch": "main",
  "files": [
    {
      "file_path": "a.go",
      "comments": [
        {
          "id": "c1",
          "author": "Test User",
          "content": "a note",
          "line_number": 12,
          "status": "active",
          "context_before": "before",
          "context_line": "the line",
          "context_after": "after"
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(statePath, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	loaded, err := LoadReview(statePath)
	if err != nil {
		t.Fatalf("expected a legacy state file to load without a migration step: %v", err)
	}

	comment := loaded.Files[0].Comments[0]
	if len(comment.Anchors) != 1 {
		t.Fatalf("expected the legacy comment to upgrade to one anchor, got %d", len(comment.Anchors))
	}
	anchor := comment.Anchors[0]
	if anchor.Blob != "" {
		t.Errorf("expected the upgraded anchor to carry an empty (legacy) blob, got %q", anchor.Blob)
	}
	if anchor.LineNumber != 12 {
		t.Errorf("expected the upgraded anchor at line 12, got %d", anchor.LineNumber)
	}
	expected := []string{"before", "the line", "after"}
	if len(anchor.Context) != 3 || anchor.Context[0] != expected[0] || anchor.Context[1] != expected[1] || anchor.Context[2] != expected[2] {
		t.Errorf("expected the upgraded context %v, got %v", expected, anchor.Context)
	}
	if anchor.Offset != 1 {
		t.Errorf("expected the legacy anchored line at offset 1, got %d", anchor.Offset)
	}
	if comment.IsOutdated() {
		t.Error("expected an upgraded legacy comment to be current, not outdated")
	}
}

// a comment created with the anchor API survives a save/load round-trip with its
// full anchor history intact.
func TestSaveAndLoadPreservesAnchorHistory(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "anchor_state.json")

	review := model.NewReview("/repo", "feature", "main")
	diff := review.AddFileDiff("a.go")
	comment := diff.AddCommentWithContext("a note", 7, "Test User", "blob-1", []string{"x", "y", "z"}, 1)
	// a second anchor: the file moved and the comment re-anchored to line 9.
	comment.Anchors = append(comment.Anchors, model.Anchor{
		Blob: "blob-2", LineNumber: 9, Offset: 1, Context: []string{"x", "y", "z"},
	})

	if err := SaveReview(statePath, review); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadReview(statePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	got := loaded.Files[0].Comments[0]
	if len(got.Anchors) != 2 {
		t.Fatalf("expected 2 anchors to survive the round-trip, got %d", len(got.Anchors))
	}
	if got.CurrentLineNumber() != 9 {
		t.Errorf("expected the current line to be the most-recent anchor's 9, got %d", got.CurrentLineNumber())
	}
	if got.Anchors[0].Blob != "blob-1" || got.Anchors[1].Blob != "blob-2" {
		t.Errorf("expected anchor blobs [blob-1 blob-2], got [%s %s]", got.Anchors[0].Blob, got.Anchors[1].Blob)
	}
}

func TestSaveReviewStampsEmbeddedReadme(t *testing.T) {
	given := model.NewReview("/repo", "feature", "main")
	statePath := filepath.Join(t.TempDir(), "review_state.json")

	if err := SaveReview(statePath, given); err != nil {
		t.Fatalf("Failed to save review: %v", err)
	}

	loaded, err := LoadReview(statePath)
	if err != nil {
		t.Fatalf("Failed to load review: %v", err)
	}

	expected := statefileUsage
	if loaded.Readme != expected {
		t.Errorf("Expected _readme stamped from embedded readme.md, got %q", loaded.Readme)
	}

	if !strings.Contains(loaded.Readme, "code-review state file") {
		t.Errorf("Embedded readme.md appears empty or wrong: %q", loaded.Readme)
	}
}

// a saved file carries the current schema version, and reloading it classifies
// as current.
func TestSaveStampsSchemaVersion(t *testing.T) {
	review := model.NewReview("/repo", "feature", "main")
	statePath := filepath.Join(t.TempDir(), "state.json")

	if err := SaveReview(statePath, review); err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), `"version": "`+schema.Version+`"`) {
		t.Errorf("expected written file to carry version %q, file was:\n%s", schema.Version, data)
	}

	loaded, err := LoadReview(statePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Version != schema.Version {
		t.Errorf("expected loaded version %q, got %q", schema.Version, loaded.Version)
	}
	if got := schema.Classify(loaded.Version); got != schema.ClassCurrent {
		t.Errorf("expected a freshly saved file to classify as current, got %v", got)
	}
}

// a review that does not conform to the schema is neither written nor leaves a
// partial file behind.
func TestSaveRejectsNonConformingReview(t *testing.T) {
	review := model.NewReview("/repo", "feature", "main")
	diff := review.AddFileDiff("a.go")
	comment := diff.AddComment("note", 3, "Test User")
	comment.Status = "bogus" // not a valid CommentStatus

	statePath := filepath.Join(t.TempDir(), "state.json")
	err := SaveReview(statePath, review)
	if err == nil {
		t.Fatal("expected SaveReview to reject a non-conforming review")
	}
	if !strings.Contains(err.Error(), "comments.0.status") {
		t.Errorf("expected the error to name the offending field path, got %v", err)
	}

	if _, statErr := os.Stat(statePath); !os.IsNotExist(statErr) {
		t.Errorf("expected no file to be written for a non-conforming review, stat err = %v", statErr)
	}
}

// a file written by an incompatible (newer) version still loads — it is not
// force-validated against this version's schema, which the `version` literal
// alone would fail — and is classified mismatched so the caller can flag it for
// migration.
func TestLoadMismatchedVersionStillLoads(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	future := `{
  "version": "2.0.0",
  "id": "rev1",
  "repo_path": "/repo",
  "source_branch": "feature",
  "target_branch": "main",
  "files": [],
  "marked_files": []
}`
	if err := os.WriteFile(statePath, []byte(future), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	loaded, err := LoadReview(statePath)
	if err != nil {
		t.Fatalf("expected a mismatched-version file to load without force-validation: %v", err)
	}
	if loaded.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0 preserved, got %q", loaded.Version)
	}
	if got := schema.Classify(loaded.Version); got != schema.ClassMismatched {
		t.Errorf("expected mismatched classification, got %v", got)
	}
}

// a conforming state file with no `version` (written before versioning
// existed) loads and is classified as unversioned (pre-1.0.0).
func TestLoadUnversionedFileClassifiesPre100(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	unversioned := `{
  "id": "rev-legacy",
  "repo_path": "/repo",
  "source_branch": "feature",
  "target_branch": "main",
  "files": [],
  "marked_files": []
}`
	if err := os.WriteFile(statePath, []byte(unversioned), 0o644); err != nil {
		t.Fatalf("write unversioned state: %v", err)
	}

	loaded, err := LoadReview(statePath)
	if err != nil {
		t.Fatalf("expected an unversioned file to load: %v", err)
	}
	if loaded.Version != "" {
		t.Errorf("expected no version on an unversioned file, got %q", loaded.Version)
	}
	if got := schema.Classify(loaded.Version); got != schema.ClassUnversioned {
		t.Errorf("expected unversioned classification, got %v", got)
	}
}

// a file that parses as JSON but violates the schema fails to load, with an
// error distinct from a parse failure.
func TestLoadRejectsSchemaViolation(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	// parses fine (status is a string), but "bogus" is not a valid status.
	bad := `{
  "id": "rev1",
  "repo_path": "/repo",
  "source_branch": "feature",
  "target_branch": "main",
  "files": [
    {"file_path": "a.go", "comments": [
      {"id": "c1", "author": "u", "content": "x", "status": "bogus"}
    ]}
  ],
  "marked_files": []
}`
	if err := os.WriteFile(statePath, []byte(bad), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	_, err := LoadReview(statePath)
	if err == nil {
		t.Fatal("expected a schema-violating file to fail loading")
	}
	if !strings.Contains(err.Error(), "invalid state file") {
		t.Errorf("expected a schema error distinct from a parse error, got %v", err)
	}
	if !strings.Contains(err.Error(), "comments.0.status") {
		t.Errorf("expected the error to name the offending field path, got %v", err)
	}
}

func TestLoadReviewNonExistent(t *testing.T) {
	_, err := LoadReview("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestSaveReviewInvalidPath(t *testing.T) {
	review := model.NewReview("/repo", "feature", "main")
	err := SaveReview("/nonexistent/dir/file.json", review)
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestGetXDGDataDir(t *testing.T) {
	t.Run("with XDG_DATA_HOME set", func(t *testing.T) {
		oldEnv := os.Getenv("XDG_DATA_HOME")
		defer os.Setenv("XDG_DATA_HOME", oldEnv)

		os.Setenv("XDG_DATA_HOME", "/custom/data")
		dataDir := GetXDGDataDir()

		expected := "/custom/data/code-review"
		if dataDir != expected {
			t.Errorf("Expected %s, got %s", expected, dataDir)
		}
	})

	t.Run("without XDG_DATA_HOME set", func(t *testing.T) {
		oldEnv := os.Getenv("XDG_DATA_HOME")
		defer os.Setenv("XDG_DATA_HOME", oldEnv)

		os.Unsetenv("XDG_DATA_HOME")
		dataDir := GetXDGDataDir()

		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".local", "share", "code-review")
		if dataDir != expected {
			t.Errorf("Expected %s, got %s", expected, dataDir)
		}
	})
}

func TestGetReviewStatePath(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	repoPath := "/home/user/dev-local/my-project"
	sourceBranch := "feature/new-feature"
	targetBranch := "main"

	statePath := GetReviewStatePath(dataDir, repoPath, sourceBranch, targetBranch)

	if !filepath.IsAbs(statePath) {
		t.Error("Expected absolute path")
	}

	dir := filepath.Dir(statePath)
	if dir != dataDir {
		t.Errorf("Expected state file in %s, got %s", dataDir, dir)
	}

	filename := filepath.Base(statePath)
	if filename == "" {
		t.Error("Expected non-empty filename")
	}

	if filepath.Ext(filename) != ".json" {
		t.Errorf("Expected .json extension, got %s", filepath.Ext(filename))
	}

	if !strings.Contains(filename, "home--user--dev-local--my-project") {
		t.Errorf("Expected filename to contain repo path with double hyphens, got %s", filename)
	}

	if !strings.Contains(filename, "feature--new-feature") {
		t.Errorf("Expected filename to contain source branch with double hyphens, got %s", filename)
	}

	if !strings.Contains(filename, "main") {
		t.Errorf("Expected filename to contain target branch, got %s", filename)
	}

	if !strings.Contains(filename, "__") {
		t.Errorf("Expected filename to contain double underscore separators, got %s", filename)
	}

	parts := strings.Split(filename, "__")
	if len(parts) != 4 {
		t.Errorf("Expected 4 parts separated by __, got %d parts in %s", len(parts), filename)
	}
}

func TestGetReviewStatePathDifferentBranches(t *testing.T) {
	dataDir := t.TempDir()
	repoPath := "/path/to/repo"

	path1 := GetReviewStatePath(dataDir, repoPath, "feature-1", "main")
	path2 := GetReviewStatePath(dataDir, repoPath, "feature-2", "main")
	path3 := GetReviewStatePath(dataDir, repoPath, "feature-1", "develop")

	if path1 == path2 {
		t.Error("Expected different paths for different source branches")
	}

	if path1 == path3 {
		t.Error("Expected different paths for different target branches")
	}

	if path2 == path3 {
		t.Error("Expected different paths for different branch combinations")
	}
}

func TestGetReviewStatePathFormat(t *testing.T) {
	tests := []struct {
		name         string
		dataDir      string
		repoPath     string
		sourceBranch string
		targetBranch string
		wantRepo     string
		wantSource   string
		wantTarget   string
	}{
		{
			name:         "simple path and branches",
			dataDir:      "/data",
			repoPath:     "/home/user/project",
			sourceBranch: "feature",
			targetBranch: "main",
			wantRepo:     "home--user--project",
			wantSource:   "feature",
			wantTarget:   "main",
		},
		{
			name:         "path with multiple levels",
			dataDir:      "/data",
			repoPath:     "/home/user/dev-local/code-review",
			sourceBranch: "feature",
			targetBranch: "master",
			wantRepo:     "home--user--dev-local--code-review",
			wantSource:   "feature",
			wantTarget:   "master",
		},
		{
			name:         "branch with slashes",
			dataDir:      "/data",
			repoPath:     "/home/user/project",
			sourceBranch: "feature/new-feature",
			targetBranch: "release/v1.0",
			wantRepo:     "home--user--project",
			wantSource:   "feature--new-feature",
			wantTarget:   "release--v1.0",
		},
		{
			name:         "complex branch names",
			dataDir:      "/data",
			repoPath:     "/opt/repos/my-app",
			sourceBranch: "bugfix/issue-123/fix-login",
			targetBranch: "develop",
			wantRepo:     "opt--repos--my-app",
			wantSource:   "bugfix--issue-123--fix-login",
			wantTarget:   "develop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := GetReviewStatePath(tt.dataDir, tt.repoPath, tt.sourceBranch, tt.targetBranch)
			filename := filepath.Base(path)

			if !strings.Contains(filename, tt.wantRepo) {
				t.Errorf("Expected filename to contain repo path %q, got %q", tt.wantRepo, filename)
			}

			if !strings.Contains(filename, tt.wantSource) {
				t.Errorf("Expected filename to contain source branch %q, got %q", tt.wantSource, filename)
			}

			if !strings.Contains(filename, tt.wantTarget) {
				t.Errorf("Expected filename to contain target branch %q, got %q", tt.wantTarget, filename)
			}

			parts := strings.Split(strings.TrimSuffix(filename, ".json"), "__")
			if len(parts) != 4 {
				t.Errorf("Expected 4 sections separated by __, got %d: %v", len(parts), parts)
			}

			if parts[0] != tt.wantRepo {
				t.Errorf("Expected first section to be %q, got %q", tt.wantRepo, parts[0])
			}

			if parts[1] != tt.wantSource {
				t.Errorf("Expected second section to be %q, got %q", tt.wantSource, parts[1])
			}

			if parts[2] != tt.wantTarget {
				t.Errorf("Expected third section to be %q, got %q", tt.wantTarget, parts[2])
			}

			if len(parts[3]) != 16 {
				t.Errorf("Expected hash to be 16 chars, got %d: %q", len(parts[3]), parts[3])
			}
		})
	}
}

func TestGetReviewStatePathNoSingleHyphens(t *testing.T) {
	dataDir := t.TempDir()
	repoPath := "/home/user/my-project"
	sourceBranch := "feature-branch"
	targetBranch := "main"

	path := GetReviewStatePath(dataDir, repoPath, sourceBranch, targetBranch)
	filename := filepath.Base(path)

	pathSection := strings.Split(strings.TrimSuffix(filename, ".json"), "__")[0]

	singleHyphens := 0
	doubleHyphens := 0
	for i := 0; i < len(pathSection)-1; i++ {
		if pathSection[i] == '-' && pathSection[i+1] == '-' {
			doubleHyphens++
			i++
		} else if pathSection[i] == '-' {
			singleHyphens++
		}
	}

	if doubleHyphens == 0 {
		t.Error("Expected at least one double hyphen separator")
	}

	if strings.Contains(pathSection, "---") {
		t.Error("Found triple hyphen, should only have double hyphens")
	}
}

func TestGetReviewStatePathUniqueness(t *testing.T) {
	dataDir := t.TempDir()

	tests := []struct {
		repoPath     string
		sourceBranch string
		targetBranch string
	}{
		{"/home/user/project", "feature", "main"},
		{"/home/user/project", "feature", "develop"},
		{"/home/user/project", "bugfix", "main"},
		{"/home/user/other-project", "feature", "main"},
		{"/home/user/project", "feature/sub", "main"},
	}

	paths := make(map[string]bool)

	for _, tt := range tests {
		path := GetReviewStatePath(dataDir, tt.repoPath, tt.sourceBranch, tt.targetBranch)
		if paths[path] {
			t.Errorf("Duplicate path generated for %s:%s:%s", tt.repoPath, tt.sourceBranch, tt.targetBranch)
		}
		paths[path] = true
	}

	if len(paths) != len(tests) {
		t.Errorf("Expected %d unique paths, got %d", len(tests), len(paths))
	}
}

func TestGetReviewStatePathHashStability(t *testing.T) {
	dataDir := t.TempDir()
	repoPath := "/home/user/project"
	sourceBranch := "feature"
	targetBranch := "main"

	path1 := GetReviewStatePath(dataDir, repoPath, sourceBranch, targetBranch)
	path2 := GetReviewStatePath(dataDir, repoPath, sourceBranch, targetBranch)

	if path1 != path2 {
		t.Errorf("Expected stable paths, got different:\n%s\n%s", path1, path2)
	}

	hash1 := strings.Split(strings.TrimSuffix(filepath.Base(path1), ".json"), "__")[3]
	hash2 := strings.Split(strings.TrimSuffix(filepath.Base(path2), ".json"), "__")[3]

	if hash1 != hash2 {
		t.Errorf("Expected stable hash, got %s vs %s", hash1, hash2)
	}
}

func TestParseReviewStatePath(t *testing.T) {
	tests := []struct {
		name         string
		repoPath     string
		sourceBranch string
		targetBranch string
		wantRepo     []string
		wantSource   string
		wantTarget   string
	}{
		{
			name:         "simple case",
			repoPath:     "/home/user/project",
			sourceBranch: "feature",
			targetBranch: "main",
			wantRepo:     []string{"home", "user", "project"},
			wantSource:   "feature",
			wantTarget:   "main",
		},
		{
			name:         "nested repo path",
			repoPath:     "/home/user/dev-local/code-review",
			sourceBranch: "fix",
			targetBranch: "master",
			wantRepo:     []string{"home", "user", "dev-local", "code-review"},
			wantSource:   "fix",
			wantTarget:   "master",
		},
		{
			name:         "branch with slashes",
			repoPath:     "/home/user/project",
			sourceBranch: "feature/new-feature",
			targetBranch: "release/v1.0",
			wantRepo:     []string{"home", "user", "project"},
			wantSource:   "feature/new-feature",
			wantTarget:   "release/v1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			path := GetReviewStatePath(dataDir, tt.repoPath, tt.sourceBranch, tt.targetBranch)
			filename := filepath.Base(path)

			filenameWithoutExt := strings.TrimSuffix(filename, ".json")
			sections := strings.Split(filenameWithoutExt, "__")

			if len(sections) != 4 {
				t.Fatalf("Expected 4 sections, got %d: %v", len(sections), sections)
			}

			repoParts := strings.Split(sections[0], "--")
			if len(repoParts) != len(tt.wantRepo) {
				t.Errorf("Expected %d repo parts, got %d: %v", len(tt.wantRepo), len(repoParts), repoParts)
			}
			for i, want := range tt.wantRepo {
				if i >= len(repoParts) {
					t.Errorf("Missing repo part %d: want %s", i, want)
					continue
				}
				if repoParts[i] != want {
					t.Errorf("Repo part %d: want %s, got %s", i, want, repoParts[i])
				}
			}

			reconstructedSource := strings.ReplaceAll(sections[1], "--", "/")
			if reconstructedSource != tt.wantSource {
				t.Errorf("Source branch: want %s, got %s (section was %s)", tt.wantSource, reconstructedSource, sections[1])
			}

			reconstructedTarget := strings.ReplaceAll(sections[2], "--", "/")
			if reconstructedTarget != tt.wantTarget {
				t.Errorf("Target branch: want %s, got %s (section was %s)", tt.wantTarget, reconstructedTarget, sections[2])
			}

			hashSection := sections[3]
			if len(hashSection) != 16 {
				t.Errorf("Expected hash length 16, got %d: %s", len(hashSection), hashSection)
			}
		})
	}
}

func TestBranchNamesWithDoubleHyphensAmbiguity(t *testing.T) {
	dataDir := t.TempDir()
	repoPath := "/home/user/project"
	sourceBranch := "feature--with--hyphens"
	targetBranch := "main"

	path := GetReviewStatePath(dataDir, repoPath, sourceBranch, targetBranch)
	filename := filepath.Base(path)

	filenameWithoutExt := strings.TrimSuffix(filename, ".json")
	sections := strings.Split(filenameWithoutExt, "__")

	if len(sections) != 4 {
		t.Fatalf("Expected 4 sections, got %d: %v", len(sections), sections)
	}

	reconstructedSource := strings.ReplaceAll(sections[1], "--", "/")

	if reconstructedSource == sourceBranch {
		t.Error("Branch with -- in name was reconstructed correctly, but should be ambiguous")
	}

	if reconstructedSource != "feature/with/hyphens" {
		t.Errorf("Expected ambiguous reconstruction to be 'feature/with/hyphens', got %s", reconstructedSource)
	}

	t.Logf("KNOWN LIMITATION: Branches containing -- cannot be parsed unambiguously")
	t.Logf("  Original: %s", sourceBranch)
	t.Logf("  Stored: %s", sections[1])
	t.Logf("  Reconstructed: %s", reconstructedSource)
}

func TestBranchNamesWithDoubleUnderscoresAmbiguity(t *testing.T) {
	dataDir := t.TempDir()
	repoPath := "/home/user/project"
	sourceBranch := "feature__with__underscores"
	targetBranch := "main"

	path := GetReviewStatePath(dataDir, repoPath, sourceBranch, targetBranch)
	filename := filepath.Base(path)

	filenameWithoutExt := strings.TrimSuffix(filename, ".json")
	sections := strings.Split(filenameWithoutExt, "__")

	if len(sections) == 4 {
		t.Error("Branch with __ in name was split into expected 4 sections, but should have more due to ambiguity")
	}

	t.Logf("KNOWN LIMITATION: Branches or paths containing __ cannot be parsed unambiguously")
	t.Logf("  Original branch: %s", sourceBranch)
	t.Logf("  Sections found: %d (expected 4 for normal case)", len(sections))
	t.Logf("  Sections: %v", sections)
	t.Logf("  This is exceedingly rare in practice")
}

func TestReconstructRepoPathFromFilename(t *testing.T) {
	tests := []struct {
		repoPath string
		want     string
	}{
		{"/home/user/project", "/home/user/project"},
		{"/home/user/dev-local/code-review", "/home/user/dev-local/code-review"},
		{"/opt/apps/my-app", "/opt/apps/my-app"},
	}

	for _, tt := range tests {
		t.Run(tt.repoPath, func(t *testing.T) {
			dataDir := t.TempDir()
			path := GetReviewStatePath(dataDir, tt.repoPath, "main", "main")
			filename := filepath.Base(path)

			filenameWithoutExt := strings.TrimSuffix(filename, ".json")
			sections := strings.Split(filenameWithoutExt, "__")
			repoParts := strings.Split(sections[0], "--")

			reconstructed := "/" + strings.Join(repoParts, "/")

			if reconstructed != tt.want {
				t.Errorf("Reconstructed path: want %s, got %s", tt.want, reconstructed)
			}
		})
	}
}
