package main

import (
	"code-review/model"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func SaveReview(path string, review *model.Review) error {
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal review: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write review file: %w", err)
	}

	return nil
}

func LoadReview(path string) (*model.Review, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read review file: %w", err)
	}

	var review model.Review
	if err := json.Unmarshal(data, &review); err != nil {
		return nil, fmt.Errorf("failed to unmarshal review: %w", err)
	}

	return &review, nil
}

func GetXDGDataDir() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "code-review")
}

func GetReviewStatePath(dataDir string, repoPath string, sourceBranch string, targetBranch string) string {
	cleanPath := filepath.Clean(repoPath)
	cleanPath = strings.ReplaceAll(cleanPath, string(filepath.Separator), "_")

	cleanSource := strings.ReplaceAll(sourceBranch, "/", "_")
	cleanTarget := strings.ReplaceAll(targetBranch, "/", "_")

	hash := sha256.Sum256([]byte(repoPath))
	hashStr := hex.EncodeToString(hash[:8])

	filename := fmt.Sprintf("%s_%s_%s_%s.json", cleanPath, cleanSource, cleanTarget, hashStr)

	return filepath.Join(dataDir, filename)
}
