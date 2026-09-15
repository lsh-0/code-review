package main

import (
	"code-review/model"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// maps a decoded external review document onto the review model, and merges the
// result into a review.
//
// Every function here is pure: the mapping takes the document plus the diff lines
// and blob SHAs it is placing against, and the merge takes the review and the
// mapped comments. Neither runs git or touches the filesystem, so both are tested
// against the committed fixture alone — the same discipline `ReanchorComments` and
// `EvictChangedMarks` follow.

// the provider prefix of a source id. Bitbucket is the only producer today; a
// second one normalises onto the same document format upstream and would carry its
// own prefix.
const foreignSourcePrefix = "bb"

// a comment mapped out of a document, paired with the file it belongs to. An empty
// `FilePath` marks a review-level comment, matching the empty-path surface the
// review already uses for overall feedback.
type mappedComment struct {
	FilePath string
	Comment  *model.Comment
}

// what an import did, for the reviewer or the agent that ran it.
//
// `Placed` and `Untethered` partition the imported root comments by whether their
// stated line was found in the diff; `UnplaceablePaths` names the files carrying a
// comment whose position could never have a line (an old-side or whole-file
// position), which is a property of the document rather than of local drift.
// `UnmatchedPaths` names files the document comments on that are absent from the
// current diff: their comments are imported and reach the CLI, but no GUI surface
// lists them.
//
// Both path lists name the file rather than only counting it, because a count alone
// does not tell the reviewer where to look for a comment that arrived untethered.
type importReport struct {
	Source           string
	Imported         int
	Placed           int
	Untethered       int
	Skipped          int
	Replies          int
	UnplaceablePaths []string
	UnmatchedPaths   []string
	MatchedPaths     int
}

// report whether the document commented on files but none of them appear in the
// current diff, which means it belongs to a different branch or repository.
func (r importReport) whollyUnmatched() bool {
	return r.MatchedPaths == 0 && len(r.UnmatchedPaths) > 0
}

// the report as the lines the CLI prints and the GUI shows.
func (r importReport) Lines() []string {
	lines := []string{
		fmt.Sprintf("imported %d comments from %s (%d placed, %d untethered, %d replies)",
			r.Imported, r.Source, r.Placed, r.Untethered, r.Replies),
	}
	for _, path := range r.UnplaceablePaths {
		lines = append(lines, fmt.Sprintf("addresses no line in the new file, rendered untethered: %s", path))
	}
	if r.Skipped > 0 {
		lines = append(lines, fmt.Sprintf("skipped %d deleted or pending comments", r.Skipped))
	}
	for _, path := range r.UnmatchedPaths {
		lines = append(lines, fmt.Sprintf("not in the current diff: %s", path))
	}
	if r.whollyUnmatched() {
		lines = append(lines, "no path in this document matches the current diff: it may belong to another branch")
	}
	return lines
}

// the source id for a document: a provider prefix and the pull request number.
//
// Derived from the document rather than the file it was read from, so re-importing
// the same pull request from a renamed file still matches its existing comments.
func foreignSourceID(doc *foreignDocument) string {
	return fmt.Sprintf("%s:%d", foreignSourcePrefix, doc.PullRequest.ID)
}

// the source registry record for a document, stamped with `importedAt`. The pull
// request's title is recorded in its own field as well as in the display label, so
// reading it does not mean parsing the label.
func foreignSource(doc *foreignDocument, importedAt string) model.Source {
	title := strings.TrimSpace(doc.PullRequest.Title)
	label := fmt.Sprintf("Bitbucket #%d", doc.PullRequest.ID)
	if title != "" {
		label = fmt.Sprintf("%s — %s", label, title)
	}
	return model.Source{
		ID:           foreignSourceID(doc),
		Label:        label,
		PullRequest:  doc.PullRequest.ID,
		Title:        title,
		SourceBranch: doc.PullRequest.Source.Branch,
		TargetBranch: doc.PullRequest.Destination.Branch,
		ImportedAt:   importedAt,
	}
}

// the author to attribute an imported comment to. The display name is always
// present in the format; an automated author is marked so a reviewer can tell
// machine feedback from a person's without matching names against a bot list.
func foreignAuthorName(author foreignAuthor) string {
	name := strings.TrimSpace(author.DisplayName)
	if name == "" {
		name = "unknown"
	}
	if author.Kind == authorApplication {
		return name + " (bot)"
	}
	return name
}

// the new-side line a position addresses, and whether it has one.
//
// An `updated` position addresses the lines the change added; an `unchanged`
// position is a context line present on both sides, so its new-side span applies. An
// `original` position addresses lines the change removed and a `whole_file`
// position addresses no line at all: neither has a new-side placement, so both
// report false and their comments import untethered.
func positionNewLine(position foreignPosition) (int, bool) {
	switch position.Kind {
	case positionUpdated:
		return position.Lines.First, true
	case positionUnchanged:
		return position.UpdatedLines.First, true
	default:
		return 0, false
	}
}

// map a document onto comments to merge, placing each against `diffLines` and
// `blobs` (both keyed by file path, as the recomputed diff provides them).
//
// A file-anchored comment is placed at the first line of its position's span. A
// multi-line span yields one comment at its first line, not one per line. A comment
// whose line is absent from the diff, or whose position addresses no new-side
// line, gets an adrift anchor and renders untethered. Review-level comments carry
// no anchor at all, matching a locally added one.
//
// Deleted and pending comments are skipped and counted: a deleted comment has no
// text, and a pending one is an unpublished draft the source's own reviewers cannot
// see. Skipping a root skips its replies with it.
//
// Replies flatten by re-rooting to the thread's root, which is the rule
// `AddReply` already applies to a local reply-to-a-reply. Order and per-comment
// author survive; the reply-to-a-reply relationship does not.
func mapForeignDocument(
	doc *foreignDocument,
	diffLines map[string][]model.DiffLineRef,
	blobs map[string]string,
) ([]mappedComment, importReport) {
	sourceID := foreignSourceID(doc)
	report := importReport{Source: sourceID}
	mapped := make([]mappedComment, 0)

	unmatched := make(map[string]bool)
	matched := make(map[string]bool)
	unplaceable := make(map[string]bool)

	for _, thread := range doc.threads() {
		if skipForeignComment(thread.Root) {
			report.Skipped += countForeignComments(thread.Root)
			continue
		}

		filePath := ""
		if thread.Location.Kind == locationFileRevision {
			filePath = thread.Location.Path
			if _, present := diffLines[filePath]; present {
				matched[filePath] = true
			} else {
				unmatched[filePath] = true
			}
		}

		root := foreignRootComment(thread, sourceID, diffLines[filePath], blobs[filePath])
		mapped = append(mapped, mappedComment{FilePath: filePath, Comment: root})
		report.Imported++

		if filePath != "" {
			if root.IsOutdated() {
				report.Untethered++
				if _, hasLine := positionNewLine(thread.Location.Position); !hasLine {
					// the position addresses no new-side line at all, so this is a
					// property of the document rather than of local drift. Name the
					// file: a count alone does not say where to look.
					unplaceable[filePath] = true
				}
			} else {
				report.Placed++
			}
		}

		replies, skipped := foreignReplies(thread.Root, root.ID, sourceID)
		for _, reply := range replies {
			mapped = append(mapped, mappedComment{FilePath: filePath, Comment: reply})
		}
		report.Imported += len(replies)
		report.Replies += len(replies)
		report.Skipped += skipped
	}

	report.MatchedPaths = len(matched)
	report.UnmatchedPaths = sortedKeys(unmatched)
	report.UnplaceablePaths = sortedKeys(unplaceable)
	return mapped, report
}

// build the root comment of a thread, anchored at the line its position states.
//
// A review-level thread (empty `filePath` at the caller) is given no anchor, so it
// behaves as an overall-feedback comment. A file-anchored thread gets a good anchor
// when its line is in `lines`, and an adrift one otherwise.
func foreignRootComment(
	thread foreignThread,
	sourceID string,
	lines []model.DiffLineRef,
	blob string,
) *model.Comment {
	comment := &model.Comment{
		ID:      model.GenerateID(),
		Author:  foreignAuthorName(thread.Root.Author),
		Content: thread.Root.Body.Text,
		Status:  foreignStatus(thread.Resolution),
		Origin:  &model.Origin{Source: sourceID, ForeignID: strconv.Itoa(thread.Root.ID)},
	}

	if thread.Location.Kind != locationFileRevision {
		return comment
	}

	line, hasLine := positionNewLine(thread.Location.Position)
	if !hasLine {
		// an old-side or whole-file position: no new-side line exists to anchor to.
		comment.Anchors = []model.Anchor{{Blob: blob}}
		return comment
	}

	comment.Anchors = []model.Anchor{model.AnchorAtLine(blob, line, lines)}
	return comment
}

// the local status an imported thread starts at. A thread already resolved at the
// source starts resolved, so a reviewer is not asked to work through feedback its
// author has already closed. Everything else starts active.
func foreignStatus(resolution foreignResolution) model.CommentStatus {
	switch resolution.State {
	case resolutionResolved, resolutionResolvedWithoutInfo:
		return model.CommentStatusResolved
	default:
		return model.CommentStatusActive
	}
}

// the flattened replies of `root`, each parented to `rootID`, in the order they
// were written. Returns the replies and the number of descendants skipped for
// being deleted or pending.
//
// The walk is depth-first, so a reply that answered another reply keeps its place
// in the reading order even though it is re-rooted.
func foreignReplies(root foreignComment, rootID string, sourceID string) ([]*model.Comment, int) {
	replies := make([]*model.Comment, 0)
	skipped := 0

	var walk func(comments []foreignComment)
	walk = func(comments []foreignComment) {
		for _, comment := range comments {
			if skipForeignComment(comment) {
				skipped += countForeignComments(comment)
				continue
			}
			replies = append(replies, &model.Comment{
				ID:       model.GenerateID(),
				ParentID: rootID,
				Author:   foreignAuthorName(comment.Author),
				Content:  comment.Body.Text,
				Status:   model.CommentStatusActive,
				Origin:   &model.Origin{Source: sourceID, ForeignID: strconv.Itoa(comment.ID)},
			})
			walk(comment.Replies)
		}
	}
	walk(root.Replies)

	return replies, skipped
}

// report whether a source comment carries nothing worth importing: a deleted body
// has no text, and a pending comment is an unpublished draft.
func skipForeignComment(comment foreignComment) bool {
	return comment.Body.Kind == bodyDeleted || comment.Pending
}

// the number of comments in a subtree, counting the root and every descendant. Used
// to report a skipped thread by the comments it cost, not by one.
func countForeignComments(comment foreignComment) int {
	total := 1
	for _, reply := range comment.Replies {
		total += countForeignComments(reply)
	}
	return total
}

// merge `mapped` into `review` as the imported feedback of `source`, replacing
// whatever that source contributed before.
//
// Existing imported comments are matched to incoming ones by their foreign id
// within this source. A match keeps the local comment's `ID` and `Status` and takes
// the incoming content and anchors: keeping the id is what keeps local replies
// attached, since a reply's `ParentID` names it, and keeping the status is what
// stops a locally resolved thread reverting to active. A comment absent from
// `mapped` is removed together with its local replies, because the thread no longer
// exists at the source.
//
// Only comments whose origin names `source` are touched. Local comments and other
// sources' comments are left exactly as they are.
func mergeForeignComments(review *model.Review, source model.Source, mapped []mappedComment) {
	byPath := make(map[string][]mappedComment)
	for _, entry := range mapped {
		byPath[entry.FilePath] = append(byPath[entry.FilePath], entry)
	}

	// every surface the source previously contributed to must be visited, even one
	// the incoming document no longer mentions, so its stale comments are removed.
	for _, file := range review.Files {
		if _, incoming := byPath[file.FilePath]; !incoming && !hasSourceComment(file.Comments, source.ID) {
			continue
		}
		file.Comments = mergeSurface(file.Comments, byPath[file.FilePath], source.ID)
		delete(byPath, file.FilePath)
	}

	if _, incoming := byPath[""]; incoming || hasSourceComment(review.Comments, source.ID) {
		review.Comments = mergeSurface(review.Comments, byPath[""], source.ID)
		delete(byPath, "")
	}

	// remaining paths have feedback but no `FileDiff` yet: a file the document
	// comments on that the current diff does not carry. The comments are kept so the
	// CLI can reach them, and the report names the path.
	for path, entries := range byPath {
		file := review.GetFileDiff(path)
		if file == nil {
			file = review.AddFileDiff(path)
		}
		file.Comments = mergeSurface(file.Comments, entries, source.ID)
	}

	review.UpsertSource(source)
}

// merge one surface's comments: keep every comment not belonging to `sourceID`,
// carry forward the local id and status of each matched imported comment, and drop
// the source's comments that the incoming set no longer has.
func mergeSurface(existing []*model.Comment, incoming []mappedComment, sourceID string) []*model.Comment {
	// the surviving imported roots of this source, by foreign id, so a re-imported
	// root keeps the id its local replies point at.
	previous := make(map[string]*model.Comment)
	for _, comment := range existing {
		if comment.SourceID() == sourceID && comment.ParentID == "" && comment.Origin != nil {
			previous[comment.Origin.ForeignID] = comment
		}
	}

	// adopt the stored id and status of each re-imported root. Keeping the id is what
	// keeps local replies attached, since a reply's `ParentID` names it; keeping the
	// status is what stops a locally resolved thread reverting to active.
	//
	// A root's id changing here has to carry to the incoming replies parented to it,
	// which were built against its freshly generated id. `reparented` records the
	// swap so they follow their root rather than being orphaned.
	reparented := make(map[string]string)
	survivingRoots := make(map[string]bool)
	for _, entry := range incoming {
		comment := entry.Comment
		if comment.ParentID != "" || comment.Origin == nil {
			continue
		}
		if kept, found := previous[comment.Origin.ForeignID]; found {
			reparented[comment.ID] = kept.ID
			comment.ID = kept.ID
			comment.Status = kept.Status
		}
		survivingRoots[comment.ID] = true
	}
	for _, entry := range incoming {
		comment := entry.Comment
		if comment.ParentID == "" {
			continue
		}
		if adopted, swapped := reparented[comment.ParentID]; swapped {
			comment.ParentID = adopted
		}
	}

	merged := make([]*model.Comment, 0, len(existing)+len(incoming))
	for _, comment := range existing {
		// another source's comment, or a local root: untouched.
		if comment.SourceID() != sourceID {
			// a local reply belongs to the thread it answers. It survives only while
			// that thread does.
			if comment.ParentID != "" && !survivingRoots[comment.ParentID] && isSourceThread(existing, comment.ParentID, sourceID) {
				continue
			}
			merged = append(merged, comment)
			continue
		}
		// this source's own comments are replaced wholesale by the incoming set.
	}
	for _, entry := range incoming {
		merged = append(merged, entry.Comment)
	}
	return merged
}

// report whether `parentID` names a root belonging to `sourceID` within `comments`.
// Distinguishes a local reply whose imported thread is going away from one whose
// root is local and unaffected.
func isSourceThread(comments []*model.Comment, parentID string, sourceID string) bool {
	for _, comment := range comments {
		if comment.ID == parentID {
			return comment.SourceID() == sourceID
		}
	}
	return false
}

// report whether any comment in `comments` was imported from `sourceID`.
func hasSourceComment(comments []*model.Comment, sourceID string) bool {
	for _, comment := range comments {
		if comment.SourceID() == sourceID {
			return true
		}
	}
	return false
}

// read and decode an external review document from `path`.
//
// Reads the file without modifying it. Returns an error naming the path when it
// cannot be read, and the decoder's error when the contents are not a valid
// document; the caller writes nothing in either case.
func loadForeignDocument(path string) (*foreignDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	doc, err := decodeForeignDocument(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

// map `doc` and merge it into `review` as one source, returning what it did.
//
// Pure: `diffLines` and `blobs` are the placement inputs (keyed by file path, as the
// recomputed diff provides them) and nothing here reads git or the filesystem. The
// caller persists the review; this leaves it in memory.
func importForeignReview(
	review *model.Review,
	doc *foreignDocument,
	diffLines map[string][]model.DiffLineRef,
	blobs map[string]string,
	importedAt string,
) importReport {
	mapped, report := mapForeignDocument(doc, diffLines, blobs)
	mergeForeignComments(review, foreignSource(doc, importedAt), mapped)
	return report
}

// the keys of `set` in sorted order, so a report lists paths deterministically.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
