package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git user email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git user name: %v", err)
	}

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add test file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	return tmpDir
}

func TestGetGitRoot(t *testing.T) {
	t.Run("valid git repo", func(t *testing.T) {
		tmpDir := setupTestRepo(t)

		root, err := GetGitRoot(tmpDir)
		if err != nil {
			t.Errorf("Expected to get git root, got error: %v", err)
		}

		if root != tmpDir {
			t.Errorf("Expected git root to be %s, got %s", tmpDir, root)
		}
	})

	t.Run("subdirectory of git repo", func(t *testing.T) {
		tmpDir := setupTestRepo(t)
		subDir := filepath.Join(tmpDir, "subdir")
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}

		root, err := GetGitRoot(subDir)
		if err != nil {
			t.Errorf("Expected to get git root from subdirectory, got error: %v", err)
		}

		if root != tmpDir {
			t.Errorf("Expected git root to be %s, got %s", tmpDir, root)
		}
	})

	t.Run("not a git repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := GetGitRoot(tmpDir)
		if err == nil {
			t.Error("Expected error for non-git directory")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := GetGitRoot("/nonexistent/path")
		if err == nil {
			t.Error("Expected error for nonexistent directory")
		}
	})
}

func TestIsGitRepo(t *testing.T) {
	t.Run("valid git repo", func(t *testing.T) {
		tmpDir := setupTestRepo(t)
		if !IsGitRepo(tmpDir) {
			t.Error("Expected directory to be a git repo")
		}
	})

	t.Run("not a git repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		if IsGitRepo(tmpDir) {
			t.Error("Expected directory not to be a git repo")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		if IsGitRepo("/nonexistent/path") {
			t.Error("Expected nonexistent directory not to be a git repo")
		}
	})
}

func TestGetCurrentBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)

	branch, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get current branch: %v", err)
	}

	expectedBranches := []string{"master", "main"}
	found := slices.Contains(expectedBranches, branch)

	if !found {
		t.Errorf("Expected branch to be one of %v, got %s", expectedBranches, branch)
	}
}

func TestGetDefaultBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)

	defaultBranch, err := GetDefaultBranch(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get default branch: %v", err)
	}

	expectedBranches := []string{"master", "main"}
	found := slices.Contains(expectedBranches, defaultBranch)

	if !found {
		t.Errorf("Expected default branch to be one of %v, got %s", expectedBranches, defaultBranch)
	}
}

func TestGetDiff(t *testing.T) {
	tmpDir := setupTestRepo(t)

	baseBranch, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get base branch: %v", err)
	}

	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create feature branch: %v", err)
	}

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content\nmodified line\n"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add modified file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Modify test file")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	currentBranch, _ := GetCurrentBranch(tmpDir)

	t.Logf("Base branch: %s", baseBranch)
	t.Logf("Current branch: %s", currentBranch)

	diff, err := GetDiff(tmpDir, baseBranch, currentBranch)
	if err != nil {
		t.Fatalf("Failed to get diff: %v", err)
	}

	t.Logf("Diff output: %q", diff)

	if len(diff) == 0 {
		t.Error("Expected non-empty diff")
	}

	if !contains(diff, "test.txt") {
		t.Error("Expected diff to contain test.txt")
	}

	if !contains(diff, "modified line") {
		t.Error("Expected diff to contain modified line")
	}
}

// a pure rename is reported as a rename even when the repo has rename detection
// turned off, because GetDiff passes -M. Without the flag git would emit an
// unrelated delete-plus-add pair and ParseDiff would never set Renamed.
func TestGetDiffPureRenameWithRenamesDisabled(t *testing.T) {
	tmpDir := setupTestRepo(t)

	cmd := exec.Command("git", "config", "diff.renames", "false")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to disable rename detection: %v", err)
	}

	baseBranch, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("failed to get base branch: %v", err)
	}

	for _, args := range [][]string{
		{"checkout", "-b", "feature"},
		{"mv", "test.txt", "moved.txt"},
		{"commit", "-m", "pure rename"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	headBranch, _ := GetCurrentBranch(tmpDir)
	diff, err := GetDiff(tmpDir, baseBranch, headBranch)
	if err != nil {
		t.Fatalf("failed to get diff: %v", err)
	}

	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	file := files[0]
	if !file.Renamed {
		t.Errorf("expected the file to be flagged renamed; diff was: %q", diff)
	}
	if file.Path != "moved.txt" || file.OldPath != "test.txt" {
		t.Errorf("expected rename test.txt -> moved.txt, got %q -> %q", file.OldPath, file.Path)
	}
	if len(file.Hunks) != 0 {
		t.Errorf("expected a pure rename to have 0 hunks, got %d", len(file.Hunks))
	}
}

func TestGetDiffInvalidRepo(t *testing.T) {
	_, err := GetDiff("/nonexistent", "main", "feature")
	if err == nil {
		t.Error("Expected error for nonexistent repo")
	}
}

func TestGetFileAtRevision(t *testing.T) {
	tmp_dir := setupTestRepo(t)

	branch, err := GetCurrentBranch(tmp_dir)
	if err != nil {
		t.Fatalf("Failed to get branch: %v", err)
	}

	t.Run("existing file at revision", func(t *testing.T) {
		given := "initial content\n"
		actual, err := GetFileAtRevision(tmp_dir, branch, "test.txt")
		if err != nil {
			t.Fatalf("Failed to read file at revision: %v", err)
		}
		if actual != given {
			t.Errorf("expected %q, got %q", given, actual)
		}
	})

	t.Run("missing path at revision", func(t *testing.T) {
		_, err := GetFileAtRevision(tmp_dir, branch, "nonexistent.txt")
		if err == nil {
			t.Error("expected error for missing path, got nil")
		}
	})
}

func TestGetUserName(t *testing.T) {
	tmpDir := setupTestRepo(t)

	userName, err := GetUserName(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get user name: %v", err)
	}

	if userName != "Test User" {
		t.Errorf("Expected user name 'Test User', got %s", userName)
	}
}

func TestGetUserNameInvalidRepo(t *testing.T) {
	_, err := GetUserName("/nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent repo")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != "" && substr != "" &&
		(s == substr || len(s) >= len(substr) && s[:len(substr)] == substr ||
			len(s) > len(substr) && findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGetRemoteURL(t *testing.T) {
	tmp_dir := setupTestRepo(t)

	if _, err := GetRemoteURL(tmp_dir, "origin"); err == nil || !strings.Contains(err.Error(), `"origin"`) {
		t.Errorf("expected an error naming the missing remote, got %v", err)
	}

	expected := "git@bitbucket.org:acme/router.git"
	if out, err := exec.Command("git", "-C", tmp_dir, "remote", "add", "origin", expected).CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v: %s", err, out)
	}

	actual, err := GetRemoteURL(tmp_dir, "origin")
	if err != nil || actual != expected {
		t.Errorf("got %q, %v; want %q", actual, err, expected)
	}
}
