package feedback

import (
	"fmt"
	"time"
)

// checks a document for invariants that need nothing but the document: no JSON, no
// network, no provider. Returns the findings in report order.
//
// Distinct from decoding. Decoding asks whether a document has the shape the schema
// defines, and rejects one that does not. These checks ask whether well-shaped
// feedback is internally consistent, and report what is not without rejecting it.
//
// The count reconciliation compares the comments present, less those with no body,
// against the count the source reported. A dropped page is the usual cause of a
// mismatch.
func Check(doc Document) []Finding {
	findings := make([]Finding, 0)
	findings = append(findings, duplicateIdentifiers(doc)...)
	findings = append(findings, partition(doc)...)
	findings = append(findings, emptyAnchorPaths(doc)...)
	findings = append(findings, chronology(doc)...)
	findings = append(findings, counts(doc)...)
	findings = append(findings, resolutions(doc)...)
	return Sort(findings)
}

// parses an RFC 3339 timestamp, with or without fractional seconds.
func ParseTimestamp(text string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, text)
}

// every comment in the document, roots and replies alike, in thread order then
// depth first.
func Comments(doc Document) []Comment {
	all := make([]Comment, 0)
	var walk func(comment Comment)
	walk = func(comment Comment) {
		all = append(all, comment)
		for _, reply := range comment.Replies {
			walk(reply)
		}
	}
	for _, thread := range doc.Threads() {
		walk(thread.Root)
	}
	return all
}

// a repeated identifier means a subtree was duplicated, which would also inflate
// every count derived from the document.
func duplicateIdentifiers(doc Document) []Finding {
	findings := make([]Finding, 0)
	seen := make(map[int]bool)
	for _, comment := range Comments(doc) {
		if seen[comment.ID] {
			findings = append(findings, Finding{Error, DuplicateCommentID,
				fmt.Sprintf("comment identifier %d appears more than once in the result", comment.ID)})
			continue
		}
		seen[comment.ID] = true
	}
	return findings
}

// a thread in the collection its location contradicts. The importer classifies by
// location, so this does not misplace feedback, but it shows the producer
// partitioned wrongly.
func partition(doc Document) []Finding {
	findings := make([]Finding, 0)
	for _, thread := range doc.Discussion {
		if thread.Location.Kind != LocationPullRequest {
			findings = append(findings, misplaced(thread))
		}
	}
	for _, thread := range doc.FileFeedback {
		if thread.Location.Kind == LocationPullRequest {
			findings = append(findings, misplaced(thread))
		}
	}
	return findings
}

func misplaced(thread Thread) Finding {
	return Finding{Error, PartitionViolation,
		fmt.Sprintf("thread rooted at comment %d is in the wrong collection", thread.Root.ID)}
}

// a file-anchored thread with no path addresses no file. The schema rejects a blank
// path, so this guards a document built in memory.
func emptyAnchorPaths(doc Document) []Finding {
	findings := make([]Finding, 0)
	for _, thread := range doc.Threads() {
		if thread.Location.Kind == LocationFileRevision && thread.Location.Path == "" {
			findings = append(findings, Finding{Error, InlineWithoutPath,
				fmt.Sprintf("thread rooted at comment %d is anchored to an empty path", thread.Root.ID)})
		}
	}
	return findings
}

// a reply older than the comment it answers means bad data or a tree rebuilt
// wrongly. A timestamp that does not parse is left to the schema and to
// normalisation, which report it.
func chronology(doc Document) []Finding {
	findings := make([]Finding, 0)
	var walk func(parent Comment)
	walk = func(parent Comment) {
		parentTime, parentErr := ParseTimestamp(parent.CreatedAt)
		for _, child := range parent.Replies {
			childTime, childErr := ParseTimestamp(child.CreatedAt)
			if parentErr == nil && childErr == nil && childTime.Before(parentTime) {
				findings = append(findings, Finding{Warning, ReplyPrecedesParent,
					fmt.Sprintf("reply %d was created before its parent %d", child.ID, parent.ID)})
			}
			walk(child)
		}
	}
	for _, thread := range doc.Threads() {
		walk(thread.Root)
	}
	return findings
}

// reconciles the comments present against the count the source reported. The
// source's own count excludes deleted comments, so present less bodyless should
// equal it.
func counts(doc Document) []Finding {
	all := Comments(doc)
	present := len(all)
	deleted := 0
	for _, comment := range all {
		if comment.Body.Kind == BodyDeleted {
			deleted++
		}
	}
	expected := present - deleted
	reported := doc.PullRequest.ReportedCommentCount

	findings := make([]Finding, 0)
	if expected != reported {
		findings = append(findings, Finding{Warning, CommentCountMismatch,
			fmt.Sprintf("%d comments retrieved less %d without a body is %d, but the pull request reports %d",
				present, deleted, expected, reported)})
	}
	if deleted > 0 {
		findings = append(findings, Finding{Debug, DeletedCommentsPresent,
			fmt.Sprintf("%d of %d comments have no body", deleted, present)})
	}
	return findings
}

// a resolved thread whose resolution withholds by whom and when. The cause is
// provider-specific, so the detail names only the thread. A provider's normaliser
// reports the cause and its remedy.
func resolutions(doc Document) []Finding {
	findings := make([]Finding, 0)
	for _, thread := range doc.Threads() {
		if thread.Resolution.State == ResolutionResolvedWithoutInfo {
			findings = append(findings, Finding{Warning, ResolutionDetailMissing,
				fmt.Sprintf("thread rooted at comment %d is resolved but carries no detail", thread.Root.ID)})
		}
	}
	return findings
}
