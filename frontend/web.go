//go:build js
// +build js

package main

import (
	"code-review/model"
	"encoding/json"
	"fmt"
	"github.com/gopherjs/gopherjs/js"
)

var (
	doc               *js.Object
	win               *js.Object
	currentFile       string
	currentLineNumber int
	currentCommentID  string
	currentUser       string
	currentReplyID    string
	diffFiles         []DiffFile
	commentsCache     map[string][]*model.Comment
	markedFiles       map[string]bool
	zoomLevel         = 1.0
	overviewActive    bool
	reviewCommentMode bool
)

const (
	zoomMin  = 0.5
	zoomMax  = 3.0
	zoomStep = 0.1
)

// apply the current `zoomLevel` to the document root as the `--zoom` CSS
// custom property, which scales the diff and code text.
func applyZoom() {
	if zoomLevel < zoomMin {
		zoomLevel = zoomMin
	}
	if zoomLevel > zoomMax {
		zoomLevel = zoomMax
	}
	doc.Get("documentElement").Get("style").Call("setProperty", "--zoom", fmt.Sprintf("%g", zoomLevel))
}

type DiffFile struct {
	Path   string     `json:"Path"`
	Hunks  []DiffHunk `json:"Hunks"`
	Binary bool       `json:"Binary"`
}

type DiffHunk struct {
	OldStart int        `json:"OldStart"`
	OldLines int        `json:"OldLines"`
	NewStart int        `json:"NewStart"`
	NewLines int        `json:"NewLines"`
	Lines    []DiffLine `json:"Lines"`
}

type DiffLine struct {
	Type      int    `json:"Type"`
	Content   string `json:"Content"`
	OldLineNo int    `json:"OldLineNo"`
	NewLineNo int    `json:"NewLineNo"`
}

const (
	LineContext = 0
	LineAdded   = 1
	LineRemoved = 2
)

func loadReviewInfo() {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("GetReviewInfo")
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		if len(args) > 0 && args[0] != js.Undefined {
			infoJSON := args[0].String()
			var info map[string]string
			json.Unmarshal([]byte(infoJSON), &info)

			currentUser = info["current_user"]

			branchInfo := doc.Call("getElementById", "branch-info")
			branchInfo.Set("textContent", info["source_branch"]+" → "+info["target_branch"])
		}
		return nil
	}))
}

// fetch the AI prompt (path + instructions) from the backend and write it to
// the clipboard, giving brief feedback on the button itself.
func copyStatePrompt() {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("GetStatePrompt")
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		if len(args) == 0 || args[0] == js.Undefined {
			return nil
		}
		prompt := args[0].String()

		btn := doc.Call("getElementById", "copy-prompt-btn")
		original := btn.Get("textContent").String()

		clipboard := win.Get("navigator").Get("clipboard")
		if clipboard == js.Undefined {
			return nil
		}
		writePromise := clipboard.Call("writeText", prompt)
		writePromise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			btn.Set("textContent", "Copied")
			win.Call("setTimeout", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
				btn.Set("textContent", original)
				return nil
			}), 1500)
			return nil
		}))
		return nil
	}))
}

func loadAllComments(callback func()) {
	remaining := len(diffFiles)
	if remaining == 0 {
		callback()
		return
	}

	for _, file := range diffFiles {
		filePath := file.Path
		loadComments(filePath, func() {
			remaining--
			if remaining == 0 {
				callback()
			}
		})
	}
}

// fetch the parsed diff into `diffFiles`, then load comments and marked files,
// then invoke `callback`. Used both for the initial load and on refresh, so the
// file list reflects newly committed files.
func loadDiffFiles(callback func()) {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("GetDiffFiles")
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		if len(args) > 0 && args[0] != js.Undefined {
			filesJSON := args[0].String()
			json.Unmarshal([]byte(filesJSON), &diffFiles)
			loadAllComments(func() {
				loadMarkedFiles(func() {
					callback()
				})
			})
		}
		return nil
	}))
}

// load the set of files the reviewer has marked as done into `markedFiles`,
// then invoke `callback`.
func loadMarkedFiles(callback func()) {
	markedFiles = make(map[string]bool)

	backend := win.Get("go")
	if backend == js.Undefined {
		callback()
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		callback()
		return
	}

	promise := app.Call("GetMarkedFiles")
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		if len(args) > 0 && args[0] != js.Undefined {
			var paths []string
			json.Unmarshal([]byte(args[0].String()), &paths)
			for _, path := range paths {
				markedFiles[path] = true
			}
		}
		callback()
		return nil
	}))
}

// persist the marked/unmarked state of `filePath` to the backend state file.
func setFileMarked(filePath string, marked bool) {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	app.Call("SetFileMarked", filePath, marked)
}

func getFileCommentStatus(filePath string) string {
	comments, ok := commentsCache[filePath]
	if !ok || len(comments) == 0 {
		return "none"
	}

	hasActive := false
	hasIgnored := false
	allResolved := true
	rootCount := 0

	for _, comment := range comments {
		// replies carry no meaningful status; only root comments determine the
		// file's aggregate state.
		if comment.ParentID != "" {
			continue
		}
		rootCount++
		if comment.Status == model.CommentStatusActive {
			hasActive = true
			allResolved = false
		} else if comment.Status == model.CommentStatusIgnored {
			hasIgnored = true
			allResolved = false
		} else if comment.Status == model.CommentStatusResolved {
			continue
		}
	}

	if hasActive {
		return "active"
	}
	if allResolved && rootCount > 0 {
		return "resolved"
	}
	if hasIgnored {
		return "ignored"
	}

	return "none"
}

func renderFileList() {
	container := doc.Call("getElementById", "files")
	container.Set("innerHTML", "")

	selectedFile := currentFile
	if selectedFile == "" && len(diffFiles) > 0 {
		selectedFile = diffFiles[0].Path
	}

	for _, file := range diffFiles {
		fileItem := doc.Call("createElement", "div")
		fileItem.Get("classList").Call("add", "file-item")

		status := getFileCommentStatus(file.Path)
		if status != "none" {
			fileItem.Get("classList").Call("add", "has-comments-"+status)
		}

		if file.Path == selectedFile {
			fileItem.Get("classList").Call("add", "active")
		}

		filePath := file.Path

		checkbox := doc.Call("createElement", "input")
		checkbox.Call("setAttribute", "type", "checkbox")
		checkbox.Get("classList").Call("add", "file-marked")
		checkbox.Set("checked", markedFiles[filePath])
		// clicking the checkbox must not bubble to the file-item's click
		// listener, which would select the file.
		checkbox.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			args[0].Call("stopPropagation")
			return nil
		}))
		checkbox.Call("addEventListener", "change", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			marked := this.Get("checked").Bool()
			markedFiles[filePath] = marked
			setFileMarked(filePath, marked)
			return nil
		}))
		fileItem.Call("appendChild", checkbox)

		fileName := doc.Call("createElement", "div")
		fileName.Get("classList").Call("add", "file-name")
		fileName.Set("textContent", file.Path)
		fileItem.Get("dataset").Set("path", file.Path)
		fileItem.Call("appendChild", fileName)

		fileItem.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			selectFile(filePath)
			return nil
		}))

		// the double-click below would otherwise form a word selection on the
		// filename. cancelling `selectstart` stops that selection before it
		// paints, avoiding a highlight flash.
		fileItem.Call("addEventListener", "selectstart", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			args[0].Call("preventDefault")
			return nil
		}))

		// double-clicking the item toggles its done checkbox. the constituent
		// single clicks select the file first, so a double-click acts on the
		// now-selected file. drive the checkbox itself so its `change` handler
		// remains the single place that updates state and persists.
		fileItem.Call("addEventListener", "dblclick", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			checkbox.Set("checked", !checkbox.Get("checked").Bool())
			event := win.Get("Event").New("change")
			checkbox.Call("dispatchEvent", event)
			return nil
		}))

		container.Call("appendChild", fileItem)
	}

	renderOverviewEntry()

	if currentFile == "" && !overviewActive && len(diffFiles) > 0 {
		selectFile(diffFiles[0].Path)
	}
}

// render the review-overview entry into its footer, fixed at the bottom of the
// file list so it stays in place while the file list scrolls. It gathers every
// file's feedback into one pane, carries no marked checkbox, and is highlighted
// while the overview is shown.
func renderOverviewEntry() {
	footer := doc.Call("getElementById", "overview-footer")
	footer.Set("innerHTML", "")

	entry := doc.Call("createElement", "div")
	entry.Get("classList").Call("add", "file-item")
	entry.Get("classList").Call("add", "overview-item")
	if overviewActive {
		entry.Get("classList").Call("add", "active")
	}

	label := doc.Call("createElement", "div")
	label.Get("classList").Call("add", "file-name")
	label.Set("textContent", "Review overview")
	entry.Call("appendChild", label)

	entry.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		selectOverview()
		return nil
	}))

	footer.Call("appendChild", entry)
}

func refreshState(callback func()) {
	backend := win.Get("go")
	if backend == js.Undefined {
		callback()
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		callback()
		return
	}

	promise := app.Call("RefreshState")
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		callback()
		return nil
	}))
}

func selectFile(filePath string) {
	// re-selecting the current file is a no-op: re-rendering would reset the
	// scroll position and discard expanded context, making it look like a fresh
	// page load when the viewer has not moved. The double-click-to-mark flow
	// relies on this — its leading single clicks land here and must not disturb
	// the view. The dblclick handler toggles the checkbox independently.
	if filePath == currentFile && !overviewActive {
		return
	}

	overviewActive = false
	currentFile = filePath

	diffView := doc.Call("getElementById", "diff-view")
	diffView.Set("scrollTop", 0)

	allItems := doc.Call("querySelectorAll", ".file-item")
	for i := 0; i < allItems.Length(); i++ {
		allItems.Index(i).Get("classList").Call("remove", "active")
	}

	allItems = doc.Call("querySelectorAll", ".file-item")
	for i := 0; i < allItems.Length(); i++ {
		item := allItems.Index(i)
		if item.Get("dataset").Get("path").String() == filePath {
			item.Get("classList").Call("add", "active")
			break
		}
	}

	doc.Call("getElementById", "current-file-name").Set("textContent", filePath)
	renderBrowseLink(filePath)

	refreshState(func() {
		loadComments(filePath, func() {
			renderDiff(filePath)
		})
	})
}

// show the review overview: every file's feedback gathered into the diff pane,
// reached from the entry at the end of the file list. State is refreshed first
// so the overview reflects any comments an agent left since the last view.
func selectOverview() {
	overviewActive = true
	currentFile = ""

	diffView := doc.Call("getElementById", "diff-view")
	diffView.Set("scrollTop", 0)

	allItems := doc.Call("querySelectorAll", ".file-item")
	for i := 0; i < allItems.Length(); i++ {
		item := allItems.Index(i)
		if item.Get("classList").Call("contains", "overview-item").Bool() {
			item.Get("classList").Call("add", "active")
		} else {
			item.Get("classList").Call("remove", "active")
		}
	}

	doc.Call("getElementById", "current-file-name").Set("textContent", "Review overview")
	removeBrowseLink()

	refreshState(func() {
		loadCommentedFiles(func(reviewComments []*model.Comment, files []commentedFile) {
			renderOverview(reviewComments, files)
		})
	})
}

// a file path paired with its comments, as returned by the backend's
// `GetCommentedFiles`.
type commentedFile struct {
	Path     string           `json:"path"`
	Comments []*model.Comment `json:"comments"`
}

// fetch every commented file and prime `commentsCache` for each, so the shared
// comment-thread rendering (which reads the cache for replies) works in the
// overview just as it does for a single file.
func loadCommentedFiles(callback func(reviewComments []*model.Comment, files []commentedFile)) {
	backend := win.Get("go")
	if backend == js.Undefined {
		callback(nil, nil)
		return
	}
	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		callback(nil, nil)
		return
	}

	if commentsCache == nil {
		commentsCache = make(map[string][]*model.Comment)
	}

	// fetch the per-file feedback, then the review-level comments, before
	// rendering. Review comments are cached under the empty-path key so the
	// shared comment handlers (which thread `filePath`) resolve their replies
	// and route status changes to the review level.
	filesPromise := app.Call("GetCommentedFiles")
	filesPromise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		var files []commentedFile
		if len(args) > 0 && args[0] != js.Undefined {
			json.Unmarshal([]byte(args[0].String()), &files)
		}
		for _, f := range files {
			commentsCache[f.Path] = f.Comments
		}

		reviewPromise := app.Call("GetReviewComments")
		reviewPromise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			var reviewComments []*model.Comment
			if len(args) > 0 && args[0] != js.Undefined {
				json.Unmarshal([]byte(args[0].String()), &reviewComments)
			}
			commentsCache[""] = reviewComments

			callback(reviewComments, files)
			return nil
		}))
		return nil
	}))
}

// render the overview into the diff pane: a review-level "General feedback"
// section (with an add control) followed by, for each commented file, a
// clickable filename header and its comment threads. Root comments only; replies
// nest within each via the shared rendering. Clicking a file header opens it.
func renderOverview(reviewComments []*model.Comment, files []commentedFile) {
	content := doc.Call("getElementById", "diff-content")
	content.Set("innerHTML", "")

	content.Call("appendChild", overviewReviewSection(reviewComments))

	for _, file := range files {
		filePath := file.Path

		section := doc.Call("createElement", "div")
		section.Get("classList").Call("add", "overview-file")

		heading := doc.Call("createElement", "div")
		heading.Get("classList").Call("add", "overview-file-header")
		heading.Set("textContent", filePath)
		heading.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			selectFile(filePath)
			return nil
		}))
		section.Call("appendChild", heading)

		// thread the root comments only; replies are nested by the shared
		// rendering, which reads them from `commentsCache` primed above.
		section.Call("appendChild", createCommentThread(filePath, rootComments(file.Comments)))

		content.Call("appendChild", section)
	}
}

// build the review-level feedback section: a heading, an "add comment" control,
// and any existing review comments threaded with the shared rendering (which
// routes their actions to the review level via the empty file path).
func overviewReviewSection(reviewComments []*model.Comment) *js.Object {
	section := doc.Call("createElement", "div")
	section.Get("classList").Call("add", "overview-file")
	section.Get("classList").Call("add", "overview-review")

	heading := doc.Call("createElement", "div")
	heading.Get("classList").Call("add", "overview-file-header")
	heading.Set("textContent", "General review comments")
	section.Call("appendChild", heading)

	addBtn := doc.Call("createElement", "button")
	addBtn.Get("classList").Call("add", "overview-add-comment")
	addBtn.Set("textContent", "Add comment")
	addBtn.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		showReviewCommentModal()
		return nil
	}))
	section.Call("appendChild", addBtn)

	if len(reviewComments) > 0 {
		// review comments are keyed under the empty path; their action handlers
		// pass "" as the file, routing status/reply/delete to the review level.
		section.Call("appendChild", createCommentThread("", rootComments(reviewComments)))
	}

	return section
}

// select the root comments (no parent) from a flat comment list.
func rootComments(comments []*model.Comment) []*model.Comment {
	roots := make([]*model.Comment, 0, len(comments))
	for _, comment := range comments {
		if comment.ParentID == "" {
			roots = append(roots, comment)
		}
	}
	return roots
}

// render (or replace) a "browse" link in the file header that opens the
// selected file in the OS-preferred application. A single `#browse-link`
// element is reused across selections.
func renderBrowseLink(filePath string) {
	header := doc.Call("getElementById", "current-file-header")

	removeBrowseLink()

	link := doc.Call("createElement", "button")
	link.Call("setAttribute", "id", "browse-link")
	link.Get("classList").Call("add", "browse-link")
	link.Set("textContent", "browse")
	link.Call("setAttribute", "title", "Open this file in the preferred application")
	link.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		browseFile(filePath)
		return nil
	}))
	header.Call("appendChild", link)
}

// remove the "browse" link from the file header if present. The overview has no
// single file to browse, so it clears the link.
func removeBrowseLink() {
	existing := doc.Call("getElementById", "browse-link")
	if existing != js.Undefined && existing != nil {
		existing.Call("remove")
	}
}

// open a file in the preferred application via the backend, reporting a
// failure without altering review state.
func browseFile(filePath string) {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("BrowseFile", filePath)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		return nil
	}))
	promise.Call("catch", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		win.Call("alert", "Could not open "+filePath)
		return nil
	}))
}

func loadComments(filePath string, callback func()) {
	backend := win.Get("go")
	if backend == js.Undefined {
		callback()
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		callback()
		return
	}

	promise := app.Call("GetComments", filePath)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		if len(args) > 0 && args[0] != js.Undefined {
			commentsJSON := args[0].String()
			var comments []*model.Comment
			json.Unmarshal([]byte(commentsJSON), &comments)

			if commentsCache == nil {
				commentsCache = make(map[string][]*model.Comment)
			}
			commentsCache[filePath] = comments

			callback()
		}
		return nil
	}))
}

func renderDiff(filePath string) {
	var file *DiffFile
	for i := range diffFiles {
		if diffFiles[i].Path == filePath {
			file = &diffFiles[i]
			break
		}
	}

	if file == nil {
		return
	}

	content := doc.Call("getElementById", "diff-content")
	content.Set("innerHTML", "")

	// binary files carry no hunks and must never have their blob fetched or
	// rendered: show a plain placeholder and stop before any hunk or
	// expansion work.
	if file.Binary {
		placeholder := doc.Call("createElement", "div")
		placeholder.Get("classList").Call("add", "binary-placeholder")
		placeholder.Set("textContent", "binary file")
		content.Call("appendChild", placeholder)
		return
	}

	var prevHunkElem *js.Object
	for i := range file.Hunks {
		hunk := file.Hunks[i]

		hunkElem := doc.Call("createElement", "div")
		hunkElem.Get("classList").Call("add", "diff-hunk")

		header := doc.Call("createElement", "div")
		header.Get("classList").Call("add", "hunk-header")
		headerText := fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines)
		header.Set("textContent", headerText)

		// the gap above this hunk: between the previous hunk's last new line
		// (or the start of the file) and this hunk's first new line. The
		// upward affordance sits above the header so revealed gap lines land
		// in file order between the affordance and the header; it shows
		// disabled when there is nothing hidden above the hunk.
		prevEnd := 0
		if i > 0 {
			prev := file.Hunks[i-1]
			prevEnd = prev.NewStart + prev.NewLines - 1
		}
		betweenHunks := i > 0
		upState := addExpandAffordance(hunkElem, filePath, expandUp, hunk, hunk.NewStart-1, prevEnd+1, false)

		// a between-hunk gap also gets a downward affordance on the previous
		// hunk, so it can be revealed from above as well as below. The two
		// controls are linked as siblings: each one's boundary is the other's
		// live frontier, so they converge on the gap and meet in the middle.
		// The upward control carries the lower hunk and header for the merge.
		if betweenHunks && prevHunkElem != nil {
			prevHunk := file.Hunks[i-1]
			downState := addExpandAffordance(prevHunkElem, filePath, expandDown, prevHunk, prevEnd+1, hunk.NewStart-1, false)
			upState.sibling = downState
			downState.sibling = upState
			upState.lowerHunk = hunkElem
			upState.lowerHeader = header
		}

		hunkElem.Call("appendChild", header)

		for _, line := range hunk.Lines {
			lineElem := createDiffLine(line, filePath)
			hunkElem.Call("appendChild", lineElem)
			appendCommentThread(hunkElem, filePath, line.NewLineNo)
		}

		// the gap below the last hunk extends to end-of-file. The exact total is
		// not known until fetched, but a last hunk whose trailing context is
		// shorter than the diff context size already reached end-of-file, so the
		// affordance is disabled up front; otherwise the first fetch disables it
		// if no further lines exist.
		if i == len(file.Hunks)-1 {
			lastEnd := hunk.NewStart + hunk.NewLines - 1
			atEOF := hunkReachedEOF(hunk.Lines)
			addExpandAffordance(hunkElem, filePath, expandDown, hunk, lastEnd+1, 0, atEOF)
		}

		content.Call("appendChild", hunkElem)
		prevHunkElem = hunkElem
	}
}

const (
	expandUp   = "up"
	expandDown = "down"
	expandStep = 20

	// the diff is generated with git's default of 3 trailing context lines, so
	// a last hunk carrying fewer than this many reached end-of-file.
	diffContextSize = 3
)

// report whether a hunk reaches the end of the file on the new side, so the
// downward expand control can be disabled up front with nothing below to
// reveal.
//
// The signal is the trailing context run: git emits up to `diffContextSize`
// unchanged lines after a hunk's last change, so a shorter run means the file
// ended. This only holds when the hunk's last line is a real new-side line
// (context or added). A hunk ending in removed lines says nothing about the
// new side — those deletions exist only on the old side, and the new file may
// continue well past the hunk — so it makes no end-of-file claim.
func hunkReachedEOF(lines []DiffLine) bool {
	if len(lines) == 0 {
		return false
	}
	if lines[len(lines)-1].Type == LineRemoved {
		return false
	}

	trailing := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i].Type != LineContext {
			break
		}
		trailing++
	}
	return trailing < diffContextSize
}

// append a comment thread for `lineNo` to `parent` when comments exist there.
func appendCommentThread(parent *js.Object, filePath string, lineNo int) {
	comments := getCommentsForLine(filePath, lineNo)
	if len(comments) > 0 {
		parent.Call("appendChild", createCommentThread(filePath, comments))
	}
}

// add an expansion affordance row to a hunk element and return its state.
// `direction` is `expandUp` (reveal lines before the hunk, inserted just above
// the hunk header) or `expandDown` (reveal lines after the hunk, appended to
// the hunk element). `frontier` is the next hidden new-line number adjacent to
// the hunk; `boundary` is the furthest hidden new line in the gap (0 for a
// trailing gap whose end is unknown until fetched). `hunk` supplies the old/new
// offset for revealed lines. The returned state lets the caller link a
// between-hunk gap's two converging affordances as siblings.
func addExpandAffordance(hunkElem *js.Object, filePath, direction string, hunk DiffHunk, frontier, boundary int, startDisabled bool) *expandState {
	row := doc.Call("createElement", "div")
	row.Get("classList").Call("add", "expand-row")

	label := "↑ expand " + fmt.Sprintf("%d", expandStep) + " lines"
	if direction == expandDown {
		label = "↓ expand " + fmt.Sprintf("%d", expandStep) + " lines"
	}
	row.Set("textContent", label)

	oldOffset := hunk.OldStart - hunk.NewStart

	// frontier moves as lines are revealed; captured by the click closure.
	state := &expandState{frontier: frontier, boundary: boundary, row: row}

	// an affordance with no hidden lines in its gap is disabled from the start.
	// A downward affordance whose boundary is unknown (0, an end-of-file gap
	// learned on first fetch) starts enabled unless the caller already knows the
	// hunk reached end-of-file.
	if (direction == expandUp && frontier < boundary) || (direction == expandDown && boundary > 0 && frontier > boundary) || startDisabled {
		disableExpandRow(row)
	}

	row.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		if row.Get("classList").Call("contains", "disabled").Bool() {
			return nil
		}
		expandGap(row, hunkElem, filePath, direction, oldOffset, state)
		return nil
	}))

	hunkElem.Call("appendChild", row)
	return state
}

// mutable per-affordance state: the next hidden line adjacent to the hunk and
// the furthest hidden line in the gap (0 = unknown trailing end).
//
// A between-hunk gap has two affordances converging on it — a downward control
// on the upper hunk and an upward control on the lower hunk. They are linked by
// `sibling`: each one's live frontier is the other's true boundary, so as one
// reveals lines the other's remaining gap shrinks, and they meet in the middle
// without overshooting or double-revealing. `row` is the affordance's own DOM
// row, so closing the gap can remove both controls.
type expandState struct {
	frontier int
	boundary int
	sibling  *expandState
	row      *js.Object
	// the upward affordance of a between-hunk gap records the lower hunk and its
	// header, so the gap can merge the two hunks regardless of which of the two
	// converging controls closed it.
	lowerHunk   *js.Object
	lowerHeader *js.Object
}

// the furthest hidden line this affordance may reveal toward. For a linked
// between-hunk affordance that is the sibling's current frontier (they converge
// on each other); otherwise it is the fixed gap boundary.
func (s *expandState) effectiveBoundary() int {
	if s.sibling != nil {
		return s.sibling.frontier
	}
	return s.boundary
}

// request the next step of context lines from the backend and splice them
// into the gap, then re-evaluate whether the affordance should remain.
func expandGap(row, hunkElem *js.Object, filePath, direction string, oldOffset int, state *expandState) {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}
	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	startNew, endNew := stepRange(direction, state)

	promise := app.Call("GetFileLines", filePath, startNew, endNew, oldOffset)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		if len(args) == 0 || args[0] == js.Undefined {
			return nil
		}
		var result struct {
			Lines    []DiffLine `json:"Lines"`
			TotalNew int        `json:"TotalNew"`
		}
		json.Unmarshal([]byte(args[0].String()), &result)
		spliceRevealed(row, hunkElem, filePath, direction, result.Lines)
		advanceState(direction, state, result.Lines, result.TotalNew)
		if gapExhausted(direction, state) {
			if state.sibling != nil {
				// a between-hunk gap, closed from either converging control:
				// merge the two hunks into one continuous block and remove both
				// controls.
				mergeBetweenHunks(state)
			} else {
				// top-of-file or end-of-file gap exhausted: keep the row in
				// place but disabled so navigation does not shift.
				disableExpandRow(row)
			}
		}
		return nil
	}))
	promise.Call("catch", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		// a failed range request (binary, missing path) disables the affordance
		// rather than leaving a control that never works.
		disableExpandRow(row)
		return nil
	}))
}

// merge the two hunks bracketing a fully revealed between-hunk gap into one
// continuous block. `state` is either of the gap's two converging affordances;
// the upward one carries the lower hunk and its header. Both affordance rows
// are removed, the lower header is hidden, and the join classes drop the visual
// separation between the two hunk boxes.
func mergeBetweenHunks(state *expandState) {
	up := state
	if up.lowerHunk == js.Undefined || up.lowerHunk == nil {
		up = state.sibling
	}
	if up == nil || up.lowerHunk == js.Undefined || up.lowerHunk == nil {
		return
	}

	if state.row != js.Undefined && state.row != nil {
		state.row.Call("remove")
	}
	if state.sibling != nil && state.sibling.row != js.Undefined && state.sibling.row != nil {
		state.sibling.row.Call("remove")
	}

	up.lowerHeader.Get("classList").Call("add", "joined-hidden")
	up.lowerHunk.Get("classList").Call("add", "joined-above")
	prev := up.lowerHunk.Get("previousElementSibling")
	if prev != js.Undefined && prev != nil {
		prev.Get("classList").Call("add", "joined-below")
	}
}

// mark an expansion affordance row disabled: it stays in place for stable
// navigation but is struck through and ignores clicks.
func disableExpandRow(row *js.Object) {
	row.Get("classList").Call("add", "disabled")
}

// compute the inclusive new-line range to request for the next step in a
// direction, clamped to the remaining gap.
func stepRange(direction string, state *expandState) (int, int) {
	boundary := state.effectiveBoundary()
	if direction == expandUp {
		endNew := state.frontier
		startNew := endNew - expandStep + 1
		if startNew < boundary {
			startNew = boundary
		}
		return startNew, endNew
	}
	startNew := state.frontier
	endNew := startNew + expandStep - 1
	if boundary > 0 && endNew > boundary {
		endNew = boundary
	}
	return startNew, endNew
}

// insert revealed lines (with their comment threads) into the gap in correct
// file order. A document fragment preserves each block's ascending order in a
// single insertion. For upward expansion the affordance row sits at the top of
// the hunk box (above the header); each block is inserted just after the row,
// so successive (further up the file, lower-numbered) blocks stack between the
// row and earlier blocks, with the header and body below them — keeping the
// whole region in order. For downward expansion the block is inserted before
// the row, which stays at the bottom of the hunk.
func spliceRevealed(row, hunkElem *js.Object, filePath, direction string, lines []DiffLine) {
	fragment := doc.Call("createDocumentFragment")
	for _, line := range lines {
		fragment.Call("appendChild", createDiffLine(line, filePath))
		thread := commentThreadOrNil(filePath, line.NewLineNo)
		if thread != nil {
			fragment.Call("appendChild", thread)
		}
	}

	if direction == expandUp {
		hunkElem.Call("insertBefore", fragment, row.Get("nextSibling"))
		return
	}
	hunkElem.Call("insertBefore", fragment, row)
}

// build a comment thread element for a line, or nil when there are none.
func commentThreadOrNil(filePath string, lineNo int) *js.Object {
	comments := getCommentsForLine(filePath, lineNo)
	if len(comments) == 0 {
		return nil
	}
	return createCommentThread(filePath, comments)
}

// move the frontier toward the gap boundary by the number of revealed lines,
// and learn the trailing boundary from the reported total on first fetch.
func advanceState(direction string, state *expandState, lines []DiffLine, totalNew int) {
	revealed := len(lines)
	if direction == expandUp {
		state.frontier -= revealed
		return
	}
	state.frontier += revealed
	if state.boundary == 0 {
		state.boundary = totalNew
	}
}

// report whether a gap has been fully revealed in the given direction. For a
// linked between-hunk gap the boundary is the sibling's frontier, so the gap
// closes when the two frontiers cross.
func gapExhausted(direction string, state *expandState) bool {
	boundary := state.effectiveBoundary()
	if direction == expandUp {
		return state.frontier < boundary
	}
	return boundary > 0 && state.frontier > boundary
}

func createDiffLine(line DiffLine, filePath string) *js.Object {
	lineElem := doc.Call("createElement", "div")
	lineElem.Get("classList").Call("add", "diff-line")

	switch line.Type {
	case LineAdded:
		lineElem.Get("classList").Call("add", "added")
	case LineRemoved:
		lineElem.Get("classList").Call("add", "removed")
	}

	numbers := doc.Call("createElement", "div")
	numbers.Get("classList").Call("add", "line-numbers")

	// Line numbers are rendered via CSS (.line-number::before { content:
	// attr(data-num) }) rather than as text content, so they are never part
	// of the selectable/copyable text when dragging across diff rows.
	oldNum := doc.Call("createElement", "div")
	oldNum.Get("classList").Call("add", "line-number")
	if line.OldLineNo > 0 {
		oldNum.Call("setAttribute", "data-num", fmt.Sprintf("%d", line.OldLineNo))
	}
	numbers.Call("appendChild", oldNum)

	newNum := doc.Call("createElement", "div")
	newNum.Get("classList").Call("add", "line-number")
	if line.NewLineNo > 0 {
		newNum.Call("setAttribute", "data-num", fmt.Sprintf("%d", line.NewLineNo))
		newNum.Get("classList").Call("add", "clickable")
		lineNo := line.NewLineNo
		newNum.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			showCommentModal(filePath, lineNo)
			return nil
		}))
	}
	numbers.Call("appendChild", newNum)

	lineElem.Call("appendChild", numbers)

	content := doc.Call("createElement", "div")
	content.Get("classList").Call("add", "line-content")
	content.Set("textContent", line.Content)
	lineElem.Call("appendChild", content)

	return lineElem
}

func getCommentsForLine(filePath string, lineNumber int) []*model.Comment {
	comments, ok := commentsCache[filePath]
	if !ok {
		return []*model.Comment{}
	}

	result := []*model.Comment{}
	for _, comment := range comments {
		// replies (ParentID set) are rendered under their root, not against a
		// line; a line thread is built only from root comments.
		if comment.ParentID == "" && comment.LineNumber == lineNumber {
			result = append(result, comment)
		}
	}
	return result
}

// collect the replies whose `ParentID` is `parentID`, in stored order.
func getReplies(filePath string, parentID string) []*model.Comment {
	comments, ok := commentsCache[filePath]
	if !ok {
		return nil
	}

	result := []*model.Comment{}
	for _, comment := range comments {
		if comment.ParentID == parentID {
			result = append(result, comment)
		}
	}
	return result
}

func createCommentThread(filePath string, comments []*model.Comment) *js.Object {
	thread := doc.Call("createElement", "div")
	thread.Get("classList").Call("add", "comment-thread")

	for _, comment := range comments {
		commentElem := createCommentElement(filePath, comment)
		thread.Call("appendChild", commentElem)
	}

	return thread
}

// build a comment-actions button with a label, optional extra css class, and
// click handler.
func actionButton(label string, cssClass string, onClick func()) *js.Object {
	btn := doc.Call("createElement", "button")
	btn.Set("textContent", label)
	if cssClass != "" {
		btn.Get("classList").Call("add", cssClass)
	}
	btn.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		onClick()
		return nil
	}))
	return btn
}

// render a comment (root or reply) with full edit/delete/reply parity. A reply
// is a comment whose `ParentID` is set: it is styled as a reply, omits the
// status badge, and offers no resolve/ignore/reactivate actions because only
// root comments carry a meaningful status. Replies are rendered after the
// content, nested by `parent_id`.
func createCommentElement(filePath string, comment *model.Comment) *js.Object {
	isReply := comment.ParentID != ""
	commentID := comment.ID

	elem := doc.Call("createElement", "div")
	elem.Get("classList").Call("add", "comment")
	if isReply {
		elem.Get("classList").Call("add", "comment-reply")
	}

	header := doc.Call("createElement", "div")
	header.Get("classList").Call("add", "comment-header")

	if !isReply {
		status := doc.Call("createElement", "span")
		status.Get("classList").Call("add", "comment-status")
		status.Get("classList").Call("add", string(comment.Status))
		status.Set("textContent", string(comment.Status))
		header.Call("appendChild", status)
	}

	if comment.Author != "" && comment.Author != currentUser {
		author := doc.Call("createElement", "span")
		author.Get("classList").Call("add", "comment-author")
		author.Set("textContent", " ("+comment.Author+")")
		header.Call("appendChild", author)
	}

	elem.Call("appendChild", header)

	content := doc.Call("createElement", "div")
	content.Get("classList").Call("add", "comment-content")
	content.Set("textContent", comment.Content)
	elem.Call("appendChild", content)

	if !isReply {
		replies := getReplies(filePath, commentID)
		if len(replies) > 0 {
			repliesElem := doc.Call("createElement", "div")
			repliesElem.Get("classList").Call("add", "comment-replies")
			for _, reply := range replies {
				repliesElem.Call("appendChild", createCommentElement(filePath, reply))
			}
			elem.Call("appendChild", repliesElem)
		}
	}

	actions := doc.Call("createElement", "div")
	actions.Get("classList").Call("add", "comment-actions")

	// Reply applies only to root comments: the thread is flat, so a reply has
	// nothing to nest under it. Replies get edit/delete parity but no status
	// actions, since only root comments carry a meaningful status.
	if !isReply {
		actions.Call("appendChild", actionButton("Reply", "", func() {
			showReplyModal(filePath, commentID)
		}))
	}
	actions.Call("appendChild", actionButton("Edit", "", func() {
		showEditCommentModal(filePath, commentID, comment.Content)
	}))

	if !isReply {
		if comment.Status == model.CommentStatusActive {
			actions.Call("appendChild", actionButton("Resolve", "", func() {
				resolveComment(filePath, commentID)
			}))
			actions.Call("appendChild", actionButton("Ignore", "", func() {
				ignoreComment(filePath, commentID)
			}))
		} else {
			actions.Call("appendChild", actionButton("Reactivate", "", func() {
				reactivateComment(filePath, commentID)
			}))
		}
	}

	actions.Call("appendChild", actionButton("Delete", "delete-btn", func() {
		deleteComment(filePath, commentID)
	}))

	elem.Call("appendChild", actions)

	return elem
}

func showCommentModal(filePath string, lineNumber int) {
	reviewCommentMode = false
	currentFile = filePath
	currentLineNumber = lineNumber

	modal := doc.Call("getElementById", "comment-modal")
	input := doc.Call("getElementById", "comment-input")
	input.Set("value", "")
	modal.Get("classList").Call("add", "active")
	input.Call("focus")
}

// open the add-comment modal for a review-level comment: overall feedback with
// no file or line anchor, created from the overview.
func showReviewCommentModal() {
	reviewCommentMode = true

	modal := doc.Call("getElementById", "comment-modal")
	input := doc.Call("getElementById", "comment-input")
	input.Set("value", "")
	modal.Get("classList").Call("add", "active")
	input.Call("focus")
}

func hideCommentModal() {
	modal := doc.Call("getElementById", "comment-modal")
	modal.Get("classList").Call("remove", "active")
}

func getLineContext(filePath string, lineNumber int) (string, string, string) {
	var file *DiffFile
	for i := range diffFiles {
		if diffFiles[i].Path == filePath {
			file = &diffFiles[i]
			break
		}
	}

	if file == nil {
		return "", "", ""
	}

	for _, hunk := range file.Hunks {
		for i, line := range hunk.Lines {
			if line.NewLineNo == lineNumber {
				contextLine := line.Content
				contextBefore := ""
				contextAfter := ""

				if i > 0 {
					contextBefore = hunk.Lines[i-1].Content
				}

				if i < len(hunk.Lines)-1 {
					contextAfter = hunk.Lines[i+1].Content
				}

				return contextBefore, contextLine, contextAfter
			}
		}
	}

	return "", "", ""
}

func refreshFileView(filePath string) {
	loadComments(filePath, func() {
		renderDiff(filePath)
		renderFileList()
	})
}

// re-render after a comment action, choosing the right surface: the overview
// when it is showing (including all review-level comment actions, which carry
// an empty file path), otherwise the single-file view.
func refreshAfterAction(filePath string) {
	if overviewActive {
		loadCommentedFiles(func(reviewComments []*model.Comment, files []commentedFile) {
			renderOverview(reviewComments, files)
			renderFileList()
		})
		return
	}
	refreshFileView(filePath)
}

func saveComment() {
	input := doc.Call("getElementById", "comment-input")
	content := input.Get("value").String()

	if content == "" {
		return
	}

	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	// a review-level comment has no file or line anchor; it is added through a
	// distinct backend call and refreshes the overview rather than a file view.
	if reviewCommentMode {
		promise := app.Call("AddReviewComment", content)
		promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			hideCommentModal()
			loadCommentedFiles(func(reviewComments []*model.Comment, files []commentedFile) {
				renderOverview(reviewComments, files)
			})
			return nil
		}))
		return
	}

	contextBefore, contextLine, contextAfter := getLineContext(currentFile, currentLineNumber)

	promise := app.Call("AddComment", currentFile, content, currentLineNumber, contextBefore, contextLine, contextAfter)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		hideCommentModal()
		refreshAfterAction(currentFile)
		return nil
	}))
}

func showEditCommentModal(filePath string, commentID string, content string) {
	currentFile = filePath
	currentCommentID = commentID

	modal := doc.Call("getElementById", "edit-comment-modal")
	input := doc.Call("getElementById", "edit-comment-input")
	input.Set("value", content)
	modal.Get("classList").Call("add", "active")
	input.Call("focus")
}

func hideEditCommentModal() {
	modal := doc.Call("getElementById", "edit-comment-modal")
	modal.Get("classList").Call("remove", "active")
}

func updateComment() {
	input := doc.Call("getElementById", "edit-comment-input")
	content := input.Get("value").String()

	if content == "" {
		return
	}

	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("UpdateComment", currentFile, currentCommentID, content)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		hideEditCommentModal()
		refreshAfterAction(currentFile)
		return nil
	}))
}

func showReplyModal(filePath string, commentID string) {
	currentFile = filePath
	currentReplyID = commentID

	modal := doc.Call("getElementById", "reply-comment-modal")
	input := doc.Call("getElementById", "reply-comment-input")
	input.Set("value", "")
	modal.Get("classList").Call("add", "active")
	input.Call("focus")
}

func hideReplyModal() {
	modal := doc.Call("getElementById", "reply-comment-modal")
	modal.Get("classList").Call("remove", "active")
}

func saveReply() {
	input := doc.Call("getElementById", "reply-comment-input")
	content := input.Get("value").String()

	if content == "" {
		return
	}

	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("AddReply", currentFile, currentReplyID, content)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		hideReplyModal()
		refreshAfterAction(currentFile)
		return nil
	}))
}

func resolveComment(filePath string, commentID string) {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("ResolveComment", filePath, commentID)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		refreshAfterAction(filePath)
		return nil
	}))
}

func ignoreComment(filePath string, commentID string) {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("IgnoreComment", filePath, commentID)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		refreshAfterAction(filePath)
		return nil
	}))
}

func reactivateComment(filePath string, commentID string) {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("ReactivateComment", filePath, commentID)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		refreshAfterAction(filePath)
		return nil
	}))
}

func deleteComment(filePath string, commentID string) {
	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("DeleteComment", filePath, commentID)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		refreshAfterAction(filePath)
		return nil
	}))
}

func setupEventHandlers() {
	doc.Call("addEventListener", "keydown", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		event := args[0]
		key := event.Get("key").String()
		if key == "Escape" || key == "Esc" {
			commentModal := doc.Call("getElementById", "comment-modal")
			editModal := doc.Call("getElementById", "edit-comment-modal")

			if commentModal.Get("classList").Call("contains", "active").Bool() {
				hideCommentModal()
			}
			if editModal.Get("classList").Call("contains", "active").Bool() {
				hideEditCommentModal()
			}
		}

		// Ctrl +/-/0 zoom the code text, mirroring the Ctrl+wheel handler.
		if event.Get("ctrlKey").Bool() {
			switch key {
			case "=", "+":
				event.Call("preventDefault")
				zoomLevel += zoomStep
				applyZoom()
			case "-", "_":
				event.Call("preventDefault")
				zoomLevel -= zoomStep
				applyZoom()
			case "0":
				event.Call("preventDefault")
				zoomLevel = 1.0
				applyZoom()
			}
		}
		return nil
	}))

	doc.Call("getElementById", "refresh-btn").Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		refreshState(func() {
			loadDiffFiles(func() {
				renderFileList()
				if overviewActive {
					loadCommentedFiles(func(reviewComments []*model.Comment, files []commentedFile) {
						renderOverview(reviewComments, files)
					})
				} else if currentFile != "" {
					renderDiff(currentFile)
				}
			})
		})
		return nil
	}))

	doc.Call("getElementById", "copy-prompt-btn").Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		copyStatePrompt()
		return nil
	}))

	doc.Call("getElementById", "save-comment-btn").Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		saveComment()
		return nil
	}))

	doc.Call("getElementById", "cancel-comment-btn").Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		hideCommentModal()
		return nil
	}))

	doc.Call("getElementById", "update-comment-btn").Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		updateComment()
		return nil
	}))

	doc.Call("getElementById", "cancel-edit-comment-btn").Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		hideEditCommentModal()
		return nil
	}))

	doc.Call("getElementById", "save-reply-btn").Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		saveReply()
		return nil
	}))

	doc.Call("getElementById", "cancel-reply-btn").Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		hideReplyModal()
		return nil
	}))

	commentModal := doc.Call("getElementById", "comment-modal")
	editModal := doc.Call("getElementById", "edit-comment-modal")
	replyModal := doc.Call("getElementById", "reply-comment-modal")

	modals := []*js.Object{commentModal, editModal, replyModal}
	for _, modal := range modals {
		if modal != js.Undefined && modal != nil {
			var mousedownTarget *js.Object

			modal.Call("addEventListener", "mousedown", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
				mousedownTarget = args[0].Get("target")
				return nil
			}))

			modal.Call("addEventListener", "mouseup", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
				mouseupTarget := args[0].Get("target")
				if mousedownTarget == this && mouseupTarget == this {
					this.Get("classList").Call("remove", "active")
				}
				mousedownTarget = nil
				return nil
			}))
		}
	}

	doc.Call("addEventListener", "wheel", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		event := args[0]
		target := event.Get("target")
		deltaY := event.Get("deltaY").Float()

		// Ctrl+wheel zooms the code text rather than scrolling. The native
		// WebKit zoom is unreliable under Wails and this handler would
		// otherwise swallow the event via preventDefault below, so zoom is
		// handled explicitly here.
		if event.Get("ctrlKey").Bool() {
			event.Call("preventDefault")
			if deltaY < 0 {
				zoomLevel += zoomStep
			} else {
				zoomLevel -= zoomStep
			}
			applyZoom()
			return nil
		}

		current := target
		var scrollableElement *js.Object
		for current != js.Undefined && current != nil {
			overflowY := win.Call("getComputedStyle", current).Call("getPropertyValue", "overflow-y").String()
			if overflowY == "auto" || overflowY == "scroll" {
				scrollHeight := current.Get("scrollHeight").Int()
				clientHeight := current.Get("clientHeight").Int()
				if scrollHeight > clientHeight {
					scrollableElement = current
					break
				}
			}
			current = current.Get("parentElement")
		}

		if scrollableElement != nil {
			event.Call("preventDefault")
			scrollableElement.Call("scrollBy", 0, deltaY)
		}

		return nil
	}), map[string]interface{}{"passive": false})
}

func initialize() {
	doc = js.Global.Get("document")
	win = js.Global

	commentsCache = make(map[string][]*model.Comment)

	loadReviewInfo()
	loadDiffFiles(func() {
		renderFileList()
	})
	setupEventHandlers()
}

func main() {
	js.Global.Set("onload", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		initialize()
		return nil
	}))
}
