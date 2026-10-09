package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetGitRoot(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	return strings.TrimSpace(string(output)), nil
}

func IsGitRepo(path string) bool {
	_, err := GetGitRoot(path)
	return err == nil
}

func GetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func GetDefaultBranch(repoPath string) (string, error) {
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", branch)
		if err := cmd.Run(); err == nil {
			return branch, nil
		}
	}
	return "", fmt.Errorf("could not find main or master branch")
}

func GetDiff(repoPath, baseBranch, headBranch string) (string, error) {
	// -M forces rename detection regardless of the user's `diff.renames` config,
	// so a pure rename is reported as a rename (with `rename from`/`rename to`)
	// rather than an unrelated delete-plus-add pair.
	cmd := exec.Command("git", "-C", repoPath, "diff", "-M", baseBranch+"..."+headBranch)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("failed to get diff: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to get diff: %w", err)
	}
	return string(output), nil
}

// read the full content of a file at a given revision via `git show
// <rev>:<path>`. Returns an error when the path does not exist at that
// revision, or when the entry is not a readable blob.
func GetFileAtRevision(repoPath, rev, path string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "show", rev+":"+path)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("failed to read %s at %s: %w (stderr: %s)", path, rev, err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to read %s at %s: %w", path, rev, err)
	}
	return string(output), nil
}

func GetUserName(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "config", "user.name")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get user name: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// read the URL of the remote `name`, such as `origin`, via `git remote get-url`.
// Read-only. Returns an error naming the remote when it is not configured.
func GetRemoteURL(repoPath, name string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", name)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no %q remote is configured", name)
	}
	return strings.TrimSpace(string(output)), nil
}
