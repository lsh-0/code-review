package feedback

import (
	"fmt"
	"sort"
	"strings"
)

// how far a finding undermines trust in a document.
//
// `Debug` is an expected condition worth recording. `Warning` is a deviation that
// leaves the document usable. `Error` means the document's structure cannot be
// trusted. None of them stops an import: a review with one malformed thread is worth
// more than no review.
type Severity int

const (
	Debug Severity = iota
	Warning
	Error
)

// the stable lowercase form, as upstream writes it.
func (s Severity) String() string {
	switch s {
	case Debug:
		return "debug"
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return fmt.Sprintf("severity(%d)", int(s))
	}
}

// what a finding is about. A closed set: the strings are upstream's stable codes, so
// a report reads the same whichever program produced it.
type Code string

const (
	OrphanReply             Code = "orphan_reply"
	ReplyCycle              Code = "reply_cycle"
	DuplicateCommentID      Code = "duplicate_comment_id"
	PartitionViolation      Code = "partition_violation"
	InlineWithoutPath       Code = "inline_without_path"
	InlineSpanInverted      Code = "inline_span_inverted"
	DeletedWithBody         Code = "deleted_with_body"
	DeletedCommentsPresent  Code = "deleted_comments_present"
	LiveCommentEmptyBody    Code = "live_comment_empty_body"
	ReplyPrecedesParent     Code = "reply_precedes_parent"
	CommentCountMismatch    Code = "comment_count_mismatch"
	UnparseableTimestamp    Code = "unparseable_timestamp"
	UnknownPullRequestState Code = "unknown_pull_request_state"
	UnknownAuthorType       Code = "unknown_author_type"
	ReplyAnchorConflict     Code = "reply_anchor_conflict"
	ResolutionOnReply       Code = "resolution_on_reply"
	ResolutionDetailMissing Code = "resolution_detail_missing"

	// not an upstream code. Upstream passes an empty display name through, which
	// the import schema rejects. The normaliser substitutes a placeholder and reports
	// it under this code instead.
	AuthorWithoutName Code = "author_without_name"

	// not an upstream code. Upstream emits an anchor naming line 0 or below as a
	// span, which the import schema rejects. The normaliser makes the position whole
	// file and reports it under this code instead.
	InlineLineOutOfRange Code = "inline_line_out_of_range"
)

// one diagnostic about a document. A value, not a log line: the code that finds it
// returns it, and the caller decides how to report it. `Detail` names the affected
// entity.
type Finding struct {
	Severity Severity
	Code     Code
	Detail   string
}

// renders as `error reply_cycle: comments 1 and 2 name each other`.
func (f Finding) String() string {
	return fmt.Sprintf("%s %s: %s", f.Severity, f.Code, f.Detail)
}

// orders by severity descending, then code, then detail, so the most serious
// finding comes first and a report is deterministic.
func less(a Finding, b Finding) bool {
	if a.Severity != b.Severity {
		return a.Severity > b.Severity
	}
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	return a.Detail < b.Detail
}

// returns `findings` in report order. Pure: the input is not modified.
func Sort(findings []Finding) []Finding {
	sorted := append([]Finding(nil), findings...)
	sort.SliceStable(sorted, func(i, j int) bool { return less(sorted[i], sorted[j]) })
	return sorted
}

// the highest severity present, and false for no findings.
func MaxSeverity(findings []Finding) (Severity, bool) {
	if len(findings) == 0 {
		return Debug, false
	}
	highest := findings[0].Severity
	for _, finding := range findings[1:] {
		if finding.Severity > highest {
			highest = finding.Severity
		}
	}
	return highest, true
}

// the findings at `minimum` severity or above, in report order.
func AtLeast(findings []Finding, minimum Severity) []Finding {
	kept := make([]Finding, 0, len(findings))
	for _, finding := range Sort(findings) {
		if finding.Severity >= minimum {
			kept = append(kept, finding)
		}
	}
	return kept
}

// one line per finding, most severe first, then a count per severity such as
// `1 error, 2 warning`. Empty for no findings. The same text upstream writes to its
// error stream.
func Report(findings []Finding) string {
	if len(findings) == 0 {
		return ""
	}
	lines := make([]string, 0, len(findings)+1)
	for _, finding := range Sort(findings) {
		lines = append(lines, finding.String())
	}
	counts := make([]string, 0, 3)
	for _, severity := range []Severity{Error, Warning, Debug} {
		n := 0
		for _, finding := range findings {
			if finding.Severity == severity {
				n++
			}
		}
		if n > 0 {
			counts = append(counts, fmt.Sprintf("%d %s", n, severity))
		}
	}
	lines = append(lines, strings.Join(counts, ", "))
	return strings.Join(lines, "\n") + "\n"
}
