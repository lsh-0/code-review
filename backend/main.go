//go:build !js

package main

import (
	"code-review/model"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"code-review/assets"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

var version = "unreleased"

type App struct {
	ctx       context.Context
	review    *model.Review
	repoPath  string
	userName  string
	dataDir   string
	statePath string
	diffFiles []DiffFile
	fileCache *fileContentCache
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

	a.repoPath, err = GetGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("repository not found: %w", err)
	}

	a.userName, err = GetUserName(a.repoPath)
	if err != nil {
		return fmt.Errorf("failed to get git user name: %w", err)
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

	fmt.Println("state:", a.statePath)

	if err := a.loadDiff(); err != nil {
		return err
	}

	return nil
}

// compute the diff between the review's branches, parse it into `diffFiles`,
// reset the file-content cache (its bodies belong to the previous diff), and
// ensure every changed file has a `FileDiff` entry in the review. Called at
// startup and again on refresh so newly committed files appear.
func (a *App) loadDiff() error {
	diffText, err := GetDiff(a.repoPath, a.review.TargetBranch, a.review.SourceBranch)
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	a.diffFiles = ParseDiff(diffText)

	a.fileCache = newFileContentCache(func(rev, path string) (string, error) {
		return GetFileAtRevision(a.repoPath, rev, path)
	})

	for _, diffFile := range a.diffFiles {
		if a.review.GetFileDiff(diffFile.Path) == nil {
			a.review.AddFileDiff(diffFile.Path)
		}
	}

	return nil
}

func (a *App) RefreshState() error {
	review, err := LoadReview(a.statePath)
	if err != nil {
		return fmt.Errorf("failed to reload state: %w", err)
	}
	a.review = review

	return a.loadDiff()
}

func (a *App) GetReviewInfo() (string, error) {
	type ReviewInfo struct {
		RepoPath     string `json:"repo_path"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		CurrentUser  string `json:"current_user"`
	}

	info := ReviewInfo{
		RepoPath:     a.review.RepoPath,
		SourceBranch: a.review.SourceBranch,
		TargetBranch: a.review.TargetBranch,
		CurrentUser:  a.userName,
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

// return a range of unchanged context lines for a file at the review's source
// (head) revision, used to expand the visible window around a hunk. The range
// [startNew, endNew] is inclusive and 1-based on new-file line numbers;
// `oldOffset` maps each new line number to its old line number (old = new +
// oldOffset). The result reports the requested lines alongside the file's
// total line count, which the viewer needs to bound the trailing gap.
func (a *App) GetFileLines(filePath string, startNew int, endNew int, oldOffset int) (string, error) {
	body, err := a.fileCache.get(a.review.SourceBranch, filePath)
	if err != nil {
		return "", err
	}

	lines, err := fileLineRange(body, startNew, endNew, oldOffset)
	if err != nil {
		return "", err
	}

	result := struct {
		Lines    []DiffLine `json:"Lines"`
		TotalNew int        `json:"TotalNew"`
	}{
		Lines:    lines,
		TotalNew: lineCount(body),
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// open a changed file in the OS-preferred application. The path is resolved
// against the repository root and opened with `xdg-open`. This does not touch
// review state; a failure to open is returned for the caller to report.
func (a *App) BrowseFile(filePath string) error {
	return OpenInPreferredApp(a.repoPath, filePath)
}

// a ready-to-paste prompt pointing a tool at the state file. The file's own
// `_readme` field carries the schema and instructions, so this stays short.
func (a *App) GetStatePrompt() string {
	return fmt.Sprintf("Read the code review state file at %s and follow the instructions in its `_readme` field: "+
		"address every comment with status 'active' (mechanical changes first, then larger ones) and set each to 'resolved' once done; "+
		"do not mark anything 'ignored' on your own — instead leave it 'active' and add a reply explaining the blocker; "+
		"and unmark any file you change by removing it from `marked_files`.", a.statePath)
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

// return the files that carry at least one comment, in diff order, each with
// its full comment array. This is the data for the review overview: a single
// call gathering every file's feedback rather than one request per file. Files
// with no comments are omitted.
func (a *App) GetCommentedFiles() (string, error) {
	type commentedFile struct {
		Path     string           `json:"path"`
		Comments []*model.Comment `json:"comments"`
	}

	result := make([]commentedFile, 0)
	for _, diffFile := range a.diffFiles {
		fileDiff := a.review.GetFileDiff(diffFile.Path)
		if fileDiff == nil || len(fileDiff.Comments) == 0 {
			continue
		}
		result = append(result, commentedFile{Path: diffFile.Path, Comments: fileDiff.Comments})
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// return the review-level comments (overall feedback, not anchored to any
// file) as JSON, for the overview to render alongside per-file feedback.
func (a *App) GetReviewComments() (string, error) {
	comments := a.review.Comments
	if comments == nil {
		comments = []*model.Comment{}
	}
	data, err := json.Marshal(comments)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) GetMarkedFiles() (string, error) {
	marked := a.review.MarkedFiles
	if marked == nil {
		marked = []string{}
	}
	data, err := json.Marshal(marked)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) SetFileMarked(filePath string, marked bool) error {
	if marked {
		a.review.MarkFile(filePath)
	} else {
		a.review.UnmarkFile(filePath)
	}

	return SaveReview(a.statePath, a.review)
}

// locate a comment by id. An empty `filePath` means a review-level comment
// (overall feedback, no file anchor); otherwise the comment is sought in that
// file's diff. Returns nil if the file or comment is absent.
func (a *App) findComment(filePath string, commentID string) *model.Comment {
	if filePath == "" {
		return a.review.GetComment(commentID)
	}
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		return nil
	}
	return fileDiff.GetComment(commentID)
}

func (a *App) AddComment(filePath string, content string, lineNumber int, contextBefore string, contextLine string, contextAfter string) error {
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		fileDiff = a.review.AddFileDiff(filePath)
	}

	fileDiff.AddCommentWithContext(content, lineNumber, a.userName, contextBefore, contextLine, contextAfter)

	return SaveReview(a.statePath, a.review)
}

// add a review-level comment: overall feedback not anchored to any file or
// line, created from the overview.
func (a *App) AddReviewComment(content string) error {
	a.review.AddComment(content, a.userName)
	return SaveReview(a.statePath, a.review)
}

func (a *App) UpdateComment(filePath string, commentID string, content string) error {
	comment := a.findComment(filePath, commentID)
	if comment == nil {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	comment.UpdateContent(content)

	return SaveReview(a.statePath, a.review)
}

func (a *App) AddReply(filePath string, commentID string, content string) error {
	if a.findComment(filePath, commentID) == nil {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	if filePath == "" {
		a.review.AddReply(commentID, content, a.userName)
	} else {
		a.review.GetFileDiff(filePath).AddReply(commentID, content, a.userName)
	}

	return SaveReview(a.statePath, a.review)
}

func (a *App) ResolveComment(filePath string, commentID string) error {
	comment := a.findComment(filePath, commentID)
	if comment == nil {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	comment.Resolve()

	return SaveReview(a.statePath, a.review)
}

func (a *App) IgnoreComment(filePath string, commentID string) error {
	comment := a.findComment(filePath, commentID)
	if comment == nil {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	comment.Ignore()

	return SaveReview(a.statePath, a.review)
}

func (a *App) ReactivateComment(filePath string, commentID string) error {
	comment := a.findComment(filePath, commentID)
	if comment == nil {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	comment.Reactivate()

	return SaveReview(a.statePath, a.review)
}

func (a *App) DeleteComment(filePath string, commentID string) error {
	if filePath == "" {
		a.review.DeleteComment(commentID)
		return SaveReview(a.statePath, a.review)
	}

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
		Bind: []any{
			app,
		},
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
