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

	for _, comment := range comments {
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
	if allResolved && len(comments) > 0 {
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

	if currentFile == "" && len(diffFiles) > 0 {
		selectFile(diffFiles[0].Path)
	}
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

// render (or replace) a "browse" link in the file header that opens the
// selected file in the OS-preferred application. A single `#browse-link`
// element is reused across selections.
func renderBrowseLink(filePath string) {
	header := doc.Call("getElementById", "current-file-header")

	existing := doc.Call("getElementById", "browse-link")
	if existing != js.Undefined && existing != nil {
		existing.Call("remove")
	}

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
		addExpandAffordance(hunkElem, header, filePath, expandUp, hunk, hunk.NewStart-1, prevEnd+1, betweenHunks)

		hunkElem.Call("appendChild", header)

		for _, line := range hunk.Lines {
			lineElem := createDiffLine(line, filePath)
			hunkElem.Call("appendChild", lineElem)
			appendCommentThread(hunkElem, filePath, line.NewLineNo)
		}

		// the gap below the last hunk extends to end-of-file. The total is not
		// known until fetched, so the affordance is always rendered; the fetch
		// disables it if no further lines exist.
		if i == len(file.Hunks)-1 {
			lastEnd := hunk.NewStart + hunk.NewLines - 1
			addExpandAffordance(hunkElem, nil, filePath, expandDown, hunk, lastEnd+1, 0, false)
		}

		content.Call("appendChild", hunkElem)
	}
}

const (
	expandUp   = "up"
	expandDown = "down"
	expandStep = 20
)

// append a comment thread for `lineNo` to `parent` when comments exist there.
func appendCommentThread(parent *js.Object, filePath string, lineNo int) {
	comments := getCommentsForLine(filePath, lineNo)
	if len(comments) > 0 {
		parent.Call("appendChild", createCommentThread(filePath, comments))
	}
}

// add an expansion affordance row to a hunk element. `direction` is `expandUp`
// (reveal lines before the hunk, inserted just above the hunk `header`) or
// `expandDown` (reveal lines after the hunk, appended to the hunk element).
// `frontier` is the next hidden new-line number adjacent to the hunk;
// `boundary` is the furthest hidden new line in the gap (0 for a trailing gap
// whose end is unknown until fetched). `hunk` supplies the old/new offset for
// revealed lines. The row is appended in render order: for an upward
// affordance it is added before the header (so it sits at the top of the box),
// for a downward affordance after the body (so it sits at the bottom).
func addExpandAffordance(hunkElem, header *js.Object, filePath, direction string, hunk DiffHunk, frontier, boundary int, betweenHunks bool) {
	row := doc.Call("createElement", "div")
	row.Get("classList").Call("add", "expand-row")

	label := "↑ expand " + fmt.Sprintf("%d", expandStep) + " lines"
	if direction == expandDown {
		label = "↓ expand " + fmt.Sprintf("%d", expandStep) + " lines"
	}
	row.Set("textContent", label)

	oldOffset := hunk.OldStart - hunk.NewStart

	// frontier moves as lines are revealed; captured by the click closure.
	state := &expandState{frontier: frontier, boundary: boundary}

	// an upward affordance with no hidden lines above is disabled from the
	// start. The downward affordance starts enabled: its end-of-file boundary
	// is only known after the first fetch.
	if direction == expandUp && frontier < boundary {
		disableExpandRow(row)
	}

	row.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		if row.Get("classList").Call("contains", "disabled").Bool() {
			return nil
		}
		expandGap(row, hunkElem, header, filePath, direction, oldOffset, state, betweenHunks)
		return nil
	}))

	hunkElem.Call("appendChild", row)
}

// mutable per-affordance state: the next hidden line adjacent to the hunk and
// the furthest hidden line in the gap (0 = unknown trailing end).
type expandState struct {
	frontier int
	boundary int
}

// request the next step of context lines from the backend and splice them
// into the gap, then re-evaluate whether the affordance should remain.
func expandGap(row, hunkElem, header *js.Object, filePath, direction string, oldOffset int, state *expandState, betweenHunks bool) {
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
			if betweenHunks && direction == expandUp {
				// a fully revealed between-hunks gap makes the two hunks one
				// continuous block: the row and this hunk's header no longer
				// mark a boundary, so remove the row, hide the header, and drop
				// the visual separation between the two boxes.
				row.Call("remove")
				header.Get("classList").Call("add", "joined-hidden")
				hunkElem.Get("classList").Call("add", "joined-above")
				prev := hunkElem.Get("previousElementSibling")
				if prev != js.Undefined && prev != nil {
					prev.Get("classList").Call("add", "joined-below")
				}
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

// mark an expansion affordance row disabled: it stays in place for stable
// navigation but is struck through and ignores clicks.
func disableExpandRow(row *js.Object) {
	row.Get("classList").Call("add", "disabled")
}

// compute the inclusive new-line range to request for the next step in a
// direction, clamped to the remaining gap.
func stepRange(direction string, state *expandState) (int, int) {
	if direction == expandUp {
		endNew := state.frontier
		startNew := endNew - expandStep + 1
		if startNew < state.boundary {
			startNew = state.boundary
		}
		return startNew, endNew
	}
	startNew := state.frontier
	endNew := startNew + expandStep - 1
	if state.boundary > 0 && endNew > state.boundary {
		endNew = state.boundary
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

// report whether a gap has been fully revealed in the given direction.
func gapExhausted(direction string, state *expandState) bool {
	if direction == expandUp {
		return state.frontier < state.boundary
	}
	return state.boundary > 0 && state.frontier > state.boundary
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
		if comment.LineNumber == lineNumber {
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

func createCommentElement(filePath string, comment *model.Comment) *js.Object {
	elem := doc.Call("createElement", "div")
	elem.Get("classList").Call("add", "comment")

	header := doc.Call("createElement", "div")
	header.Get("classList").Call("add", "comment-header")

	status := doc.Call("createElement", "span")
	status.Get("classList").Call("add", "comment-status")
	status.Get("classList").Call("add", string(comment.Status))
	status.Set("textContent", string(comment.Status))
	header.Call("appendChild", status)

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

	if len(comment.Replies) > 0 {
		replies := doc.Call("createElement", "div")
		replies.Get("classList").Call("add", "comment-replies")
		for _, reply := range comment.Replies {
			replies.Call("appendChild", createReplyElement(reply))
		}
		elem.Call("appendChild", replies)
	}

	actions := doc.Call("createElement", "div")
	actions.Get("classList").Call("add", "comment-actions")

	commentID := comment.ID

	replyBtn := doc.Call("createElement", "button")
	replyBtn.Set("textContent", "Reply")
	replyBtn.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		showReplyModal(filePath, commentID)
		return nil
	}))
	actions.Call("appendChild", replyBtn)

	editBtn := doc.Call("createElement", "button")
	editBtn.Set("textContent", "Edit")
	editBtn.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		showEditCommentModal(filePath, commentID, comment.Content)
		return nil
	}))
	actions.Call("appendChild", editBtn)

	if comment.Status == model.CommentStatusActive {
		resolveBtn := doc.Call("createElement", "button")
		resolveBtn.Set("textContent", "Resolve")
		resolveBtn.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			resolveComment(filePath, commentID)
			return nil
		}))
		actions.Call("appendChild", resolveBtn)

		ignoreBtn := doc.Call("createElement", "button")
		ignoreBtn.Set("textContent", "Ignore")
		ignoreBtn.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			ignoreComment(filePath, commentID)
			return nil
		}))
		actions.Call("appendChild", ignoreBtn)
	} else {
		reactivateBtn := doc.Call("createElement", "button")
		reactivateBtn.Set("textContent", "Reactivate")
		reactivateBtn.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			reactivateComment(filePath, commentID)
			return nil
		}))
		actions.Call("appendChild", reactivateBtn)
	}

	deleteBtn := doc.Call("createElement", "button")
	deleteBtn.Get("classList").Call("add", "delete-btn")
	deleteBtn.Set("textContent", "Delete")
	deleteBtn.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		deleteComment(filePath, commentID)
		return nil
	}))
	actions.Call("appendChild", deleteBtn)

	elem.Call("appendChild", actions)

	return elem
}

func createReplyElement(reply *model.Reply) *js.Object {
	elem := doc.Call("createElement", "div")
	elem.Get("classList").Call("add", "comment-reply")

	if reply.Author != "" && reply.Author != currentUser {
		header := doc.Call("createElement", "div")
		header.Get("classList").Call("add", "comment-header")
		author := doc.Call("createElement", "span")
		author.Get("classList").Call("add", "comment-author")
		author.Set("textContent", "("+reply.Author+")")
		header.Call("appendChild", author)
		elem.Call("appendChild", header)
	}

	content := doc.Call("createElement", "div")
	content.Get("classList").Call("add", "comment-content")
	content.Set("textContent", reply.Content)
	elem.Call("appendChild", content)

	return elem
}

func showCommentModal(filePath string, lineNumber int) {
	currentFile = filePath
	currentLineNumber = lineNumber

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

func saveComment() {
	input := doc.Call("getElementById", "comment-input")
	content := input.Get("value").String()

	if content == "" {
		return
	}

	contextBefore, contextLine, contextAfter := getLineContext(currentFile, currentLineNumber)

	backend := win.Get("go")
	if backend == js.Undefined {
		return
	}

	app := backend.Get("main").Get("App")
	if app == js.Undefined {
		return
	}

	promise := app.Call("AddComment", currentFile, content, currentLineNumber, contextBefore, contextLine, contextAfter)
	promise.Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		hideCommentModal()
		refreshFileView(currentFile)
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
		refreshFileView(currentFile)
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
		refreshFileView(currentFile)
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
		refreshFileView(filePath)
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
		refreshFileView(filePath)
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
		refreshFileView(filePath)
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
		refreshFileView(filePath)
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
				if currentFile != "" {
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
