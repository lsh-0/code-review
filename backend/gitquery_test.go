package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestGetWorkingTreeStatus_Modified(t *testing.T) {
	tmpDir := setupTestRepo(t)

	// setupTestRepo commits test.txt; modify it in the working tree.
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("changed\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	status, err := GetWorkingTreeStatus(tmpDir)
	if err != nil {
		t.Fatalf("GetWorkingTreeStatus failed: %v", err)
	}

	if !slices.Contains(status.Modified, "test.txt") {
		t.Errorf("expected test.txt reported modified, got %+v", status.Modified)
	}
	if !status.DirtyFiles["test.txt"] {
		t.Error("expected test.txt in DirtyFiles")
	}
}

func TestGetWorkingTreeStatus_Deleted(t *testing.T) {
	tmpDir := setupTestRepo(t)

	// removing a tracked file leaves it tracked-but-missing in the working tree.
	if err := os.Remove(filepath.Join(tmpDir, "test.txt")); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	status, err := GetWorkingTreeStatus(tmpDir)
	if err != nil {
		t.Fatalf("GetWorkingTreeStatus failed: %v", err)
	}

	if !slices.Contains(status.Deleted, "test.txt") {
		t.Errorf("expected test.txt reported deleted, got %+v", status.Deleted)
	}
	if !status.DirtyFiles["test.txt"] {
		t.Error("expected deleted test.txt in DirtyFiles")
	}
}

func TestGetWorkingTreeStatus_ExcludesUntracked(t *testing.T) {
	tmpDir := setupTestRepo(t)

	// a brand new file is untracked and must not be reported.
	if err := os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	status, err := GetWorkingTreeStatus(tmpDir)
	if err != nil {
		t.Fatalf("GetWorkingTreeStatus failed: %v", err)
	}

	if slices.Contains(status.Modified, "new.txt") || slices.Contains(status.Deleted, "new.txt") {
		t.Errorf("expected untracked new.txt excluded, got modified=%+v deleted=%+v", status.Modified, status.Deleted)
	}
	if status.DirtyFiles["new.txt"] {
		t.Error("expected untracked new.txt absent from DirtyFiles")
	}
}

func TestGetWorkingTreeStatus_CleanTree(t *testing.T) {
	tmpDir := setupTestRepo(t)

	status, err := GetWorkingTreeStatus(tmpDir)
	if err != nil {
		t.Fatalf("GetWorkingTreeStatus failed: %v", err)
	}

	if len(status.Modified) != 0 || len(status.Deleted) != 0 {
		t.Errorf("expected a clean tree to report nothing, got modified=%+v deleted=%+v", status.Modified, status.Deleted)
	}
}

// build a status from modified/deleted lists, deriving DirtyFiles as the
// backend does (the union of both).
func makeStatus(modified, deleted []string) WorkingTreeStatus {
	dirty := map[string]bool{}
	for _, p := range append(append([]string{}, modified...), deleted...) {
		dirty[p] = true
	}
	return WorkingTreeStatus{Modified: modified, Deleted: deleted, DirtyFiles: dirty}
}

func TestWorkingTreeStatusEqual(t *testing.T) {
	cases := []struct {
		name     string
		a        WorkingTreeStatus
		b        WorkingTreeStatus
		expected bool
	}{
		{
			name:     "identical statuses are equal",
			a:        makeStatus([]string{"a.go", "b.go"}, []string{"c.go"}),
			b:        makeStatus([]string{"a.go", "b.go"}, []string{"c.go"}),
			expected: true,
		},
		{
			name:     "order does not matter",
			a:        makeStatus([]string{"a.go", "b.go"}, []string{"c.go"}),
			b:        makeStatus([]string{"b.go", "a.go"}, []string{"c.go"}),
			expected: true,
		},
		{
			name:     "two clean trees are equal",
			a:        makeStatus([]string{}, []string{}),
			b:        makeStatus([]string{}, []string{}),
			expected: true,
		},
		{
			name:     "a differing modified set is not equal",
			a:        makeStatus([]string{"a.go"}, []string{}),
			b:        makeStatus([]string{"a.go", "b.go"}, []string{}),
			expected: false,
		},
		{
			name:     "a differing deleted set is not equal",
			a:        makeStatus([]string{"a.go"}, []string{"c.go"}),
			b:        makeStatus([]string{"a.go"}, []string{"d.go"}),
			expected: false,
		},
		{
			name:     "the same path modified vs deleted is not equal",
			a:        makeStatus([]string{"a.go"}, []string{}),
			b:        makeStatus([]string{}, []string{"a.go"}),
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := WorkingTreeStatusEqual(tc.a, tc.b)
			if actual != tc.expected {
				t.Errorf("WorkingTreeStatusEqual = %v, expected %v", actual, tc.expected)
			}
		})
	}
}

func TestBlobSHAs(t *testing.T) {
	tmpDir := setupTestRepo(t)
	branch, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("failed to get branch: %v", err)
	}

	// the test repo commits test.txt; ask for it plus a path that does not exist.
	shas, err := BlobSHAs(tmpDir, branch, []string{"test.txt", "missing.txt"})
	if err != nil {
		t.Fatalf("BlobSHAs failed: %v", err)
	}

	sha, ok := shas["test.txt"]
	if !ok || sha == "" {
		t.Errorf("expected a blob SHA for test.txt, got %q (present=%v)", sha, ok)
	}
	// a path absent at the revision is omitted, not errored.
	if _, present := shas["missing.txt"]; present {
		t.Error("expected a non-existent path to be absent from the result")
	}

	// the SHA changes when the file's content changes.
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("changed\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}
	add := exec.Command("git", "add", "test.txt")
	add.Dir = tmpDir
	add.Run()
	commit := exec.Command("git", "commit", "-m", "change")
	commit.Dir = tmpDir
	commit.Run()

	after, err := BlobSHAs(tmpDir, branch, []string{"test.txt"})
	if err != nil {
		t.Fatalf("BlobSHAs after change failed: %v", err)
	}
	if after["test.txt"] == sha {
		t.Error("expected the blob SHA to change after the file's content changed")
	}
}
