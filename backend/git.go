package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
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
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get default branch: %w", err)
		}
	}

	branch := strings.TrimSpace(string(output))
	branch = strings.TrimPrefix(branch, "refs/remotes/origin/")
	return branch, nil
}

func GetDiff(repoPath, baseBranch, headBranch string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "diff", baseBranch+"..."+headBranch)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("failed to get diff: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to get diff: %w", err)
	}
	return string(output), nil
}
