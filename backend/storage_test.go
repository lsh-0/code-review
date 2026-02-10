package main

import (
	"code-review/model"
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
	diff.AddComment("This needs refactoring", 10)
	diff.AddComment("Add error handling here", 25)

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
