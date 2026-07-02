package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// the committed branch-to-branch diff: what `RecomputeDiff` consumes. Wrapping
// `GetDiff` + `ParseDiff` behind a typed query keeps the diff axis distinct from
// the working-tree axis below, so the two never get fused the way `RefreshState`
// once fused state-reload and diff-recompute.
type DiffQuery struct {
	RepoPath string
	Base     string
	Head     string
}

// run the query, returning the parsed diff files between `Base` and `Head`.
func (q DiffQuery) Run() ([]DiffFile, error) {
	diffText, err := GetDiff(q.RepoPath, q.Base, q.Head)
	if err != nil {
		return nil, err
	}
	return ParseDiff(diffText), nil
}

// the git blob SHA of each given path at `rev`, via a single
// `git ls-tree <rev> -- <paths…>`. The blob SHA is git's content hash, so two
// versions of a file share a SHA only when their content is identical. A path
// that does not exist at `rev` (deleted, or never committed there) is simply
// absent from the returned map. Used to detect whether a marked file has
// changed since it was marked.
func BlobSHAs(repoPath, rev string, paths []string) (map[string]string, error) {
	result := map[string]string{}
	if len(paths) == 0 {
		return result, nil
	}

	args := append([]string{"-C", repoPath, "ls-tree", rev, "--"}, paths...)
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return result, fmt.Errorf("failed to read blob SHAs at %s: %w (stderr: %s)", rev, err, string(exitErr.Stderr))
		}
		return result, fmt.Errorf("failed to read blob SHAs at %s: %w", rev, err)
	}

	// each line is "<mode> SP <type> SP <sha> TAB <path>".
	for line := range strings.SplitSeq(string(output), "\n") {
		before, after, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		meta := strings.Fields(before)
		if len(meta) < 3 {
			continue
		}
		result[after] = meta[2]
	}

	return result, nil
}

// tracked files whose working-tree state differs from the index: those modified
// and those deleted. New untracked files are deliberately excluded — the review
// is about committed changes, and an untracked file is not yet part of it.
// `DirtyFiles` is the union as a set, for a per-file "is this dirty" lookup.
type WorkingTreeStatus struct {
	Modified   []string        `json:"modified"`
	Deleted    []string        `json:"deleted"`
	DirtyFiles map[string]bool `json:"dirty_files"`
}

// query the working tree for tracked files modified or deleted relative to the
// index, via `git status --porcelain`. Untracked files (`??`) are excluded. The
// porcelain format is a stable two-column status (`XY`) followed by a space and
// the path; the second column is the working-tree state, so `_M`/`MM` is a
// working-tree modification and `_D`/`MD` a working-tree deletion.
func GetWorkingTreeStatus(repoPath string) (WorkingTreeStatus, error) {
	status := WorkingTreeStatus{
		Modified:   []string{},
		Deleted:    []string{},
		DirtyFiles: map[string]bool{},
	}

	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return status, fmt.Errorf("failed to get working-tree status: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return status, fmt.Errorf("failed to get working-tree status: %w", err)
	}

	for line := range strings.SplitSeq(string(output), "\n") {
		if len(line) < 4 {
			continue
		}

		// columns 0,1 are the staged/working-tree status; the path begins at 3.
		worktree := line[1]
		path := line[3:]

		// untracked files report "??"; skip them.
		if line[0] == '?' {
			continue
		}

		switch worktree {
		case 'M':
			status.Modified = append(status.Modified, path)
			status.DirtyFiles[path] = true
		case 'D':
			status.Deleted = append(status.Deleted, path)
			status.DirtyFiles[path] = true
		}
	}

	return status, nil
}

// report whether two working-tree statuses describe the same dirty tree: the
// same modified set and the same deleted set (order-insensitive). The poller
// uses this to emit `worktree:changed` only on a real transition rather than on
// every tick. `DirtyFiles` is not compared directly: it is the derived union of
// the two sets, so equal modified and deleted sets imply an equal union.
func WorkingTreeStatusEqual(a, b WorkingTreeStatus) bool {
	return sameSet(a.Modified, b.Modified) && sameSet(a.Deleted, b.Deleted)
}

// whether two path slices contain the same paths regardless of order. Git
// reports porcelain paths in a stable order, so a plain length-and-membership
// check suffices without sorting.
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, p := range a {
		seen[p] = true
	}
	for _, p := range b {
		if !seen[p] {
			return false
		}
	}
	return true
}
