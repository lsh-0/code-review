package main

import (
	"code-review/model"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

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

	// the modification time of the state file as of this GUI's own last write,
	// so the watcher can tell a self-write (which it must ignore) from an
	// external write by another process. Guarded because the watcher polls it
	// from a separate goroutine.
	savedMu        sync.Mutex
	lastSavedMtime time.Time
}

func NewApp() *App {
	return &App{}
}

// persist the review and record the resulting file modification time, so the
// watcher does not mistake this GUI's own write for an external change. Every
// state mutation goes through here rather than calling SaveReview directly.
func (a *App) persist() error {
	if err := SaveReview(a.statePath, a.review); err != nil {
		return err
	}
	a.markSaved()
	return nil
}

// record the state file's current modification time as this GUI's last write.
func (a *App) markSaved() {
	info, err := os.Stat(a.statePath)
	if err != nil {
		return
	}
	a.savedMu.Lock()
	a.lastSavedMtime = info.ModTime()
	a.savedMu.Unlock()
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
		if err := a.persist(); err != nil {
			return fmt.Errorf("failed to save new review: %w", err)
		}
	}

	// record the baseline mtime (the file existed already, or was just written)
	// so the watcher's first comparison is against this GUI's own state.
	a.markSaved()

	fmt.Println("state:", a.statePath)

	if err := a.RecomputeDiff(); err != nil {
		return err
	}

	// watch the state file for edits by another process (an agent or the CLI),
	// so the GUI can offer a refresh when the review changes underneath it.
	go a.watchStateFile(ctx)

	// watch the working tree for uncommitted changes, so the banners update the
	// moment a tracked file changes on disk.
	go a.watchWorkingTree(ctx)

	return nil
}

// recompute the diff between the review's branches, parse it into `diffFiles`,
// reset the file-content cache (its bodies belong to the previous diff), and
// ensure every changed file has a `FileDiff` entry in the review. This is the
// only path that shells out to `git diff`. Called at startup and again on the
// explicit refresh so newly committed files appear. It is deliberately not
// called on file selection, where the diff is unchanged.
func (a *App) RecomputeDiff() error {
	newDiff, err := DiffQuery{
		RepoPath: a.repoPath,
		Base:     a.review.TargetBranch,
		Head:     a.review.SourceBranch,
	}.Run()
	if err != nil {
		return err
	}

	a.evictChangedMarks()

	a.diffFiles = newDiff

	a.fileCache = newFileContentCache(func(rev, path string) (string, error) {
		return GetFileAtRevision(a.repoPath, rev, path)
	})

	for _, diffFile := range a.diffFiles {
		if a.review.GetFileDiff(diffFile.Path) == nil {
			a.review.AddFileDiff(diffFile.Path)
		}
	}

	a.reanchorComments()

	return nil
}

// re-anchor every commented file's comments against the recomputed diff. It
// fetches each commented file's current blob SHA (the same source `evictChangedMarks`
// uses) and the new-side line contents from the freshly parsed diff, then hands
// both to the pure `Review.ReanchorComments`. A blob-SHA lookup failure skips
// re-anchoring this pass rather than marking comments adrift on a transient git
// error.
func (a *App) reanchorComments() {
	paths := make([]string, 0, len(a.review.Files))
	for _, file := range a.review.Files {
		if len(file.Comments) > 0 {
			paths = append(paths, file.FilePath)
		}
	}
	if len(paths) == 0 {
		return
	}

	blobs, err := BlobSHAs(a.repoPath, a.review.SourceBranch, paths)
	if err != nil {
		return
	}

	a.review.ReanchorComments(blobs, diffLinesByPath(a.diffFiles))
}

// the new-side diff lines of each file in `files`, keyed by path, each paired
// with its new-file line number. These are the lines a comment can re-anchor
// onto: a position absent from the diff is unreachable, so a comment whose
// context lands only outside the diff goes adrift.
func diffLinesByPath(files []DiffFile) map[string][]model.DiffLineRef {
	out := make(map[string][]model.DiffLineRef, len(files))
	for _, file := range files {
		lines := make([]model.DiffLineRef, 0)
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Type != LineRemoved {
					lines = append(lines, model.DiffLineRef{
						Content:    line.Content,
						LineNumber: line.NewLineNo,
					})
				}
			}
		}
		out[file.Path] = lines
	}
	return out
}

// drop the "done" mark from any file whose committed content has changed since
// it was marked, by comparing each mark's stored blob SHA against the file's
// current SHA at the source-branch HEAD. A file deleted at that revision is
// also unmarked; a legacy mark with no stored SHA is backfilled. This makes
// mark eviction survive restarts: it depends only on the marks and git, not on
// any prior in-session diff.
func (a *App) evictChangedMarks() {
	if len(a.review.MarkedFiles) == 0 {
		return
	}

	paths := make([]string, 0, len(a.review.MarkedFiles))
	for _, mark := range a.review.MarkedFiles {
		paths = append(paths, mark.Path)
	}

	current, err := BlobSHAs(a.repoPath, a.review.SourceBranch, paths)
	if err != nil {
		return
	}

	a.review.EvictChangedMarks(current)
}

// re-read the review state (comments, marks, replies) from disk into
// `a.review`. This is the cheap reload: it reads one JSON file and does no git
// work, so picking up an agent's edits or switching to the overview costs only
// a file read. The diff is untouched.
func (a *App) ReloadReview() error {
	review, err := LoadReview(a.statePath)
	if err != nil {
		return fmt.Errorf("failed to reload state: %w", err)
	}
	a.review = review

	return nil
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

// return the changed files as metadata only — path and binary flag, without the
// hunks. The file list needs only this to render the sidebar, and shipping every
// file's line-level hunks in one payload was the bulk of the startup transfer
// cost. The hunks for a single file are fetched on demand via `GetFileDiff` when
// it is selected.
func (a *App) GetDiffFiles() (string, error) {
	type diffFileMeta struct {
		Path   string `json:"Path"`
		Binary bool   `json:"Binary"`
	}

	metas := make([]diffFileMeta, 0, len(a.diffFiles))
	for _, diffFile := range a.diffFiles {
		metas = append(metas, diffFileMeta{Path: diffFile.Path, Binary: diffFile.Binary})
	}

	data, err := json.Marshal(metas)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// return one changed file's full diff (hunks and binary flag) as JSON, fetched
// when that file is selected. This pairs with `GetDiffFiles`, which returns only
// metadata, so the line-level content crosses the bridge one file at a time
// rather than all at once on startup. An unknown path yields a null result.
func (a *App) GetFileDiff(filePath string) (string, error) {
	for i := range a.diffFiles {
		if a.diffFiles[i].Path == filePath {
			data, err := json.Marshal(a.diffFiles[i])
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
	}
	return "null", nil
}

// return the working-tree status (tracked files modified or deleted relative to
// the index, untracked excluded) as JSON. This is distinct from the branch
// diff: it backs the uncommitted-change banners, warning the reviewer when what
// they see may not reflect what is on disk.
func (a *App) GetWorkingTreeStatus() (string, error) {
	status, err := GetWorkingTreeStatus(a.repoPath)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(status)
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

// open a file's uncommitted working-tree changes in the reviewer's configured
// diff tool via `git difftool` (honouring their `diff.tool`, e.g. meld). The
// path is resolved against the repository root. This does not touch review
// state; a failure to launch the tool is returned for the caller to report.
func (a *App) OpenDiffToolForFile(filePath string) error {
	return OpenDiffTool(a.repoPath, filePath)
}

// a ready-to-paste prompt pointing an agent at the CLI. The full contract lives
// in the `instructions` command, so this only directs the agent there.
func (a *App) GetStatePrompt() string {
	return readmePointer
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

// the marked-file paths, as a bare string array for the frontend (which only
// needs to know which files are marked, not their stored signatures).
func (a *App) GetMarkedFiles() (string, error) {
	paths := make([]string, 0, len(a.review.MarkedFiles))
	for _, mark := range a.review.MarkedFiles {
		paths = append(paths, mark.Path)
	}
	data, err := json.Marshal(paths)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) SetFileMarked(filePath string, marked bool) error {
	if marked {
		// record the file's current blob SHA at the source-branch HEAD, so a
		// later open can tell whether the file has changed since it was marked. A
		// lookup failure leaves the blob empty (it backfills on next open).
		blob := ""
		if shas, err := BlobSHAs(a.repoPath, a.review.SourceBranch, []string{filePath}); err == nil {
			blob = shas[filePath]
		}
		a.review.MarkFile(filePath, blob)
	} else {
		a.review.UnmarkFile(filePath)
	}

	return a.persist()
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

// the result of a comment mutation: enough for the frontend to patch the one
// affected thread and the file's status indicator without re-rendering the
// whole surface. `FilePath` is empty for review-level comments. `LineNumber` is
// the line the affected thread anchors (the comment's root line); it is -1 for
// a review-level thread, which has no line. `Comments` is the file's (or the
// review's) full flat comment array after the mutation, from which the frontend
// rebuilds the thread. `FileStatus` is the recomputed file pill status.
type CommentMutationResult struct {
	FilePath   string           `json:"file_path"`
	LineNumber int              `json:"line_number"`
	Comments   []*model.Comment `json:"comments"`
	FileStatus string           `json:"file_status"`
}

// the comments belonging to a surface: a file's `FileDiff.Comments`, or the
// review-level comments when `filePath` is empty.
func (a *App) commentsFor(filePath string) []*model.Comment {
	if filePath == "" {
		return a.review.Comments
	}
	if fileDiff := a.review.GetFileDiff(filePath); fileDiff != nil {
		return fileDiff.Comments
	}
	return nil
}

// assemble the mutation result for a surface, anchored at `line`. `line` is the
// affected thread's root line, computed by the caller (before a delete, since
// the comment is then gone). Review-level threads use -1.
func (a *App) mutationResult(filePath string, line int) CommentMutationResult {
	comments := a.commentsFor(filePath)
	if filePath == "" {
		line = -1
	}
	return CommentMutationResult{
		FilePath:   filePath,
		LineNumber: line,
		Comments:   comments,
		FileStatus: model.FileCommentStatus(comments),
	}
}

// save the review and return the mutation result for `filePath` at `line` as
// JSON, the shared tail of every comment mutator.
func (a *App) saveAndResult(filePath string, line int) (string, error) {
	if err := a.persist(); err != nil {
		return "", err
	}
	data, err := json.Marshal(a.mutationResult(filePath, line))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// add a line-anchored comment. `context` is the captured window of new-side
// line contents around `lineNumber` and `offset` is the anchored line's index
// within that window, used to re-anchor the comment as the file changes. The
// file's current blob SHA is captured alongside it so a later
// reconcile can tell whether the content moved; a SHA lookup failure leaves the
// anchor's blob empty (treated as a legacy baseline on first reconcile).
func (a *App) AddComment(filePath string, content string, lineNumber int, context []string, offset int) (string, error) {
	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		fileDiff = a.review.AddFileDiff(filePath)
	}

	blob := ""
	if shas, err := BlobSHAs(a.repoPath, a.review.SourceBranch, []string{filePath}); err == nil {
		blob = shas[filePath]
	}

	fileDiff.AddCommentWithContext(content, lineNumber, a.userName, blob, context, offset)

	return a.saveAndResult(filePath, lineNumber)
}

// add a review-level comment: overall feedback not anchored to any file or
// line, created from the overview.
func (a *App) AddReviewComment(content string) (string, error) {
	a.review.AddComment(content, a.userName)
	return a.saveAndResult("", -1)
}

func (a *App) UpdateComment(filePath string, commentID string, content string) (string, error) {
	comment := a.findComment(filePath, commentID)
	if comment == nil {
		return "", fmt.Errorf("comment not found: %s", commentID)
	}

	comment.UpdateContent(content)

	return a.saveAndResult(filePath, model.CommentRootLine(a.commentsFor(filePath), commentID))
}

func (a *App) AddReply(filePath string, commentID string, content string) (string, error) {
	if a.findComment(filePath, commentID) == nil {
		return "", fmt.Errorf("comment not found: %s", commentID)
	}

	if filePath == "" {
		a.review.AddReply(commentID, content, a.userName)
	} else {
		a.review.GetFileDiff(filePath).AddReply(commentID, content, a.userName)
	}

	return a.saveAndResult(filePath, model.CommentRootLine(a.commentsFor(filePath), commentID))
}

func (a *App) ResolveComment(filePath string, commentID string) (string, error) {
	comment := a.findComment(filePath, commentID)
	if comment == nil {
		return "", fmt.Errorf("comment not found: %s", commentID)
	}

	comment.Resolve()

	return a.saveAndResult(filePath, model.CommentRootLine(a.commentsFor(filePath), commentID))
}

func (a *App) IgnoreComment(filePath string, commentID string) (string, error) {
	comment := a.findComment(filePath, commentID)
	if comment == nil {
		return "", fmt.Errorf("comment not found: %s", commentID)
	}

	comment.Ignore()

	return a.saveAndResult(filePath, model.CommentRootLine(a.commentsFor(filePath), commentID))
}

func (a *App) ReactivateComment(filePath string, commentID string) (string, error) {
	comment := a.findComment(filePath, commentID)
	if comment == nil {
		return "", fmt.Errorf("comment not found: %s", commentID)
	}

	comment.Reactivate()

	return a.saveAndResult(filePath, model.CommentRootLine(a.commentsFor(filePath), commentID))
}

func (a *App) DeleteComment(filePath string, commentID string) (string, error) {
	// capture the affected thread's line before removing the comment, since the
	// root line cannot be looked up once it is gone.
	line := model.CommentRootLine(a.commentsFor(filePath), commentID)

	if filePath == "" {
		a.review.DeleteComment(commentID)
		return a.saveAndResult("", -1)
	}

	fileDiff := a.review.GetFileDiff(filePath)
	if fileDiff == nil {
		return "", fmt.Errorf("file not found: %s", filePath)
	}

	fileDiff.DeleteComment(commentID)

	return a.saveAndResult(filePath, line)
}

func main() {
	// route to the agent-facing CLI when invoked with a recognised command or a
	// help flag, before any GUI/flag handling. A bare invocation and the GUI's
	// own flags (e.g. `--version`) fall through to the GUI path below.
	if args := os.Args[1:]; isCLIInvocation(args) {
		os.Exit(runCLI(args, os.Stdout, os.Stderr))
	}

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
		// Wails disables the webview's context menu in production builds (it is
		// present only under `wails dev` and debug builds); this re-enables it so
		// the released GUI offers the native copy/cut/paste menu over a selection.
		EnableDefaultContextMenu: true,
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
