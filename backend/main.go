//go:build !js
// +build !js

package main

import (
	"code-review/model"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"code-review/assets"
)

var version = "unreleased"

type App struct {
	ctx        context.Context
	review     *model.Review
	repoPath   string
	dataDir    string
	statePath  string
	diffFiles  []DiffFile
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) error {
	a.ctx = ctx

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if !IsGitRepo(cwd) {
		return fmt.Errorf("not a git repository: %s", cwd)
	}

	a.repoPath, err = filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	a.dataDir = GetXDGDataDir()
	if err := os.MkdirAll(a.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory %s: %w", a.dataDir, err)
	}

	currentBranch, err := GetCurrentBranch(a.repoPath)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	defaultBranch, err := GetDefaultBranch(a.repoPath)
	if err != nil {
		return fmt.Errorf("failed to get default branch: %w", err)
	}

	a.statePath = GetReviewStatePath(a.dataDir, a.repoPath, currentBranch, defaultBranch)

	if _, err := os.Stat(a.statePath); err == nil {
		a.review, err = LoadReview(a.statePath)
		if err != nil {
			return fmt.Errorf("failed to load existing review: %w", err)
		}
	} else {
		a.review = model.NewReview(a.repoPath, currentBranch, defaultBranch)
		if err := SaveReview(a.statePath, a.review); err != nil {
			return fmt.Errorf("failed to save new review: %w", err)
		}
	}

	diffText, err := GetDiff(a.repoPath, a.review.TargetBranch, a.review.SourceBranch)
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	a.diffFiles = ParseDiff(diffText)

	for _, diffFile := range a.diffFiles {
		if a.review.GetFileDiff(diffFile.Path) == nil {
			a.review.AddFileDiff(diffFile.Path)
		}
	}

	return nil
}

func (a *App) GetReviewInfo() (string, error) {
	type ReviewInfo struct {
		RepoPath     string `json:"repo_path"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
	}

	info := ReviewInfo{
		RepoPath:     a.review.RepoPath,
		SourceBranch: a.review.SourceBranch,
		TargetBranch: a.review.TargetBranch,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (a *App) GetDiffFiles() (string, error) {
	data, err := json.Marshal(a.diffFiles)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) GetComments(filePath string) (string, error) {
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		return "[]", nil
	}

	data, err := json.Marshal(fileDiff.Comments)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) AddComment(filePath string, content string, lineNumber int, contextBefore string, contextLine string, contextAfter string) error {
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		fileDiff = a.review.AddFileDiff(filePath)
	}

	fileDiff.AddCommentWithContext(content, lineNumber, contextBefore, contextLine, contextAfter)

	return SaveReview(a.statePath, a.review)
}

func (a *App) UpdateComment(filePath string, commentID string, content string) error {
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	comment := fileDiff.GetComment(commentID)
	if comment == nil {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	comment.UpdateContent(content)

	return SaveReview(a.statePath, a.review)
}

func (a *App) ResolveComment(filePath string, commentID string) error {
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	comment := fileDiff.GetComment(commentID)
	if comment == nil {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	comment.Resolve()

	return SaveReview(a.statePath, a.review)
}

func (a *App) IgnoreComment(filePath string, commentID string) error {
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	comment := fileDiff.GetComment(commentID)
	if comment == nil {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	comment.Ignore()

	return SaveReview(a.statePath, a.review)
}

func (a *App) ReactivateComment(filePath string, commentID string) error {
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	comment := fileDiff.GetComment(commentID)
	if comment == nil {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	comment.Reactivate()

	return SaveReview(a.statePath, a.review)
}

func (a *App) DeleteComment(filePath string, commentID string) error {
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	fileDiff.DeleteComment(commentID)

	return SaveReview(a.statePath, a.review)
}

func main() {
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Code Review",
		Width:  1400,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets.Assets,
		},
		OnStartup: func(ctx context.Context) {
			if err := app.startup(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
				os.Exit(1)
			}
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
