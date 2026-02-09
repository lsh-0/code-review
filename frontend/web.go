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
	doc                *js.Object
	win                *js.Object
	currentFile        string
	currentLineNumber  int
	currentCommentID   string
	diffFiles          []DiffFile
	commentsCache      map[string][]*model.Comment
)

type DiffFile struct {
	Path  string     `json:"Path"`
	Hunks []DiffHunk `json:"Hunks"`
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

			branchInfo := doc.Call("getElementById", "branch-info")
			branchInfo.Set("textContent", info["source_branch"]+" → "+info["target_branch"])
		}
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

func loadDiffFiles() {
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
				renderFileList()
			})
		}
		return nil
	}))
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

	for _, file := range diffFiles {
		fileItem := doc.Call("createElement", "div")
		fileItem.Get("classList").Call("add", "file-item")

		status := getFileCommentStatus(file.Path)
		if status != "none" {
			fileItem.Get("classList").Call("add", "has-comments-"+status)
		}

		fileName := doc.Call("createElement", "div")
		fileName.Get("classList").Call("add", "file-name")
		fileName.Set("textContent", file.Path)
		fileItem.Call("appendChild", fileName)

		filePath := file.Path
		fileItem.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
			selectFile(filePath)
			return nil
		}))

		container.Call("appendChild", fileItem)
	}

	if len(diffFiles) > 0 {
		selectFile(diffFiles[0].Path)
	}
}

func selectFile(filePath string) {
	currentFile = filePath

	allItems := doc.Call("querySelectorAll", ".file-item")
	for i := 0; i < allItems.Length(); i++ {
		allItems.Index(i).Get("classList").Call("remove", "active")
	}

	allItems = doc.Call("querySelectorAll", ".file-item")
	for i := 0; i < allItems.Length(); i++ {
		item := allItems.Index(i)
		nameElem := item.Call("querySelector", ".file-name")
		if nameElem != js.Undefined && nameElem.Get("textContent").String() == filePath {
			item.Get("classList").Call("add", "active")
			break
		}
	}

	doc.Call("getElementById", "current-file-name").Set("textContent", filePath)

	loadComments(filePath, func() {
		renderDiff(filePath)
	})
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

	for _, hunk := range file.Hunks {
		hunkElem := doc.Call("createElement", "div")
		hunkElem.Get("classList").Call("add", "diff-hunk")

		header := doc.Call("createElement", "div")
		header.Get("classList").Call("add", "hunk-header")
		headerText := fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines)
		header.Set("textContent", headerText)
		hunkElem.Call("appendChild", header)

		for _, line := range hunk.Lines {
			lineElem := createDiffLine(line, filePath)
			hunkElem.Call("appendChild", lineElem)

			comments := getCommentsForLine(filePath, line.NewLineNo)
			if len(comments) > 0 {
				thread := createCommentThread(filePath, comments)
				hunkElem.Call("appendChild", thread)
			}
		}

		content.Call("appendChild", hunkElem)
	}
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

	oldNum := doc.Call("createElement", "div")
	oldNum.Get("classList").Call("add", "line-number")
	if line.OldLineNo > 0 {
		oldNum.Set("textContent", fmt.Sprintf("%d", line.OldLineNo))
	}
	numbers.Call("appendChild", oldNum)

	newNum := doc.Call("createElement", "div")
	newNum.Get("classList").Call("add", "line-number")
	if line.NewLineNo > 0 {
		newNum.Set("textContent", fmt.Sprintf("%d", line.NewLineNo))
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

	elem.Call("appendChild", header)

	content := doc.Call("createElement", "div")
	content.Get("classList").Call("add", "comment-content")
	content.Set("textContent", comment.Content)
	elem.Call("appendChild", content)

	actions := doc.Call("createElement", "div")
	actions.Get("classList").Call("add", "comment-actions")

	commentID := comment.ID

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
	deleteBtn.Set("textContent", "Delete")
	deleteBtn.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		deleteComment(filePath, commentID)
		return nil
	}))
	actions.Call("appendChild", deleteBtn)

	elem.Call("appendChild", actions)

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
		loadComments(currentFile, func() {
			renderDiff(currentFile)
		})
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
		loadComments(currentFile, func() {
			renderDiff(currentFile)
		})
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
		loadComments(filePath, func() {
			renderDiff(filePath)
		})
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
		loadComments(filePath, func() {
			renderDiff(filePath)
		})
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
		loadComments(filePath, func() {
			renderDiff(filePath)
		})
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
		loadComments(filePath, func() {
			renderDiff(filePath)
		})
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

	commentModal := doc.Call("getElementById", "comment-modal")
	editModal := doc.Call("getElementById", "edit-comment-modal")

	modals := []*js.Object{commentModal, editModal}
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
	loadDiffFiles()
	setupEventHandlers()
}

func main() {
	js.Global.Set("onload", js.MakeFunc(func(this *js.Object, args []*js.Object) interface{} {
		initialize()
		return nil
	}))
}
