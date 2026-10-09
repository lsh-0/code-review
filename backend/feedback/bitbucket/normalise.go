package bitbucket

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"code-review/feedback"
)

// translates the Bitbucket wire format onto `feedback.Document`. This is the one
// place the format's ambiguities are resolved: the flat comment list becomes a
// forest, the nullable line quadruple becomes a position, and a deletion flag with
// a body string becomes one value.
//
// Total: every structural problem becomes a finding and a document is still
// produced, because a review with one malformed anchor is worth more than no
// review.

// the display name an author with none is given. The import document requires a
// name, and Bitbucket has never been observed to omit one.
const unnamedAuthor = "unknown"

// the whitespace upstream trims, and that the schema's non-blank rule excludes.
const blank = " \t\n\r\f"

func warning(code feedback.Code, format string, args ...any) feedback.Finding {
	return feedback.Finding{Severity: feedback.Warning, Code: code, Detail: fmt.Sprintf(format, args...)}
}

func failure(code feedback.Code, format string, args ...any) feedback.Finding {
	return feedback.Finding{Severity: feedback.Error, Code: code, Detail: fmt.Sprintf(format, args...)}
}

// renders a time as the document's timestamps: UTC at microsecond precision.
func timestamp(t time.Time) string {
	return t.UTC().Format(feedback.TimestampLayout)
}

// parses `text`, or substitutes the Unix epoch with an error finding so ordering
// stays total. Every timestamp-derived conclusion is then suspect.
func parseTime(text string, what string) (time.Time, []feedback.Finding) {
	parsed, err := feedback.ParseTimestamp(text)
	if err != nil {
		return time.Unix(0, 0).UTC(), []feedback.Finding{
			failure(feedback.UnparseableTimestamp, "%s: not RFC 3339: %q", what, text),
		}
	}
	return parsed, nil
}

func optionalTime(text *string, what string) (*string, []feedback.Finding) {
	if text == nil {
		return nil, nil
	}
	parsed, findings := parseTime(*text, what)
	rendered := timestamp(parsed)
	return &rendered, findings
}

func author(user wireUser) (feedback.Author, []feedback.Finding) {
	findings := make([]feedback.Finding, 0)
	name := user.DisplayName
	if strings.Trim(name, blank) == "" {
		name = unnamedAuthor
		findings = append(findings, warning(feedback.AuthorWithoutName,
			"an author carries no display name; it is shown as %q", unnamedAuthor))
	}
	result := feedback.Author{
		DisplayName: name,
		AccountID:   user.AccountID,
		UUID:        user.UUID,
		Nickname:    user.Nickname,
	}
	switch user.Type {
	case "user":
		result.Kind = feedback.AuthorHuman
	case "app_user":
		result.Kind = feedback.AuthorApplication
		result.ApplicationKind = "app_user"
		if user.Kind != nil {
			result.ApplicationKind = *user.Kind
		}
	default:
		// not folded into human: defaulting an unrecognised author to a person would
		// misreport exactly the distinction the kind exists to draw.
		result.Kind = feedback.AuthorUnknown
		result.ReportedType = user.Type
		findings = append(findings, warning(feedback.UnknownAuthorType,
			"author %q reported type %q, which is neither user nor app_user", user.DisplayName, user.Type))
	}
	return result, findings
}

func body(id int, deleted bool, raw string) (feedback.Body, []feedback.Finding) {
	empty := strings.Trim(raw, blank) == ""
	switch {
	case deleted && empty:
		return feedback.Body{Kind: feedback.BodyDeleted}, nil
	case deleted:
		return feedback.Body{Kind: feedback.BodyDeleted}, []feedback.Finding{
			warning(feedback.DeletedWithBody, "comment %d is marked deleted but carries body text", id),
		}
	case empty:
		return feedback.Body{Kind: feedback.BodyDeleted}, []feedback.Finding{
			warning(feedback.LiveCommentEmptyBody, "comment %d is not marked deleted but has no body", id),
		}
	default:
		return feedback.Body{Kind: feedback.BodyMarkdown, Text: raw}, nil
	}
}

// a span ending at `end`, starting at `start` when one is given. A start beyond the
// end is clamped to the end line and reported, since one malformed range should not
// discard an otherwise usable review.
func span(start *int, end int, label string) (feedback.Span, []feedback.Finding) {
	if start == nil {
		return feedback.Span{First: end, Last: end}, nil
	}
	if *start > end {
		return feedback.Span{First: end, Last: end}, []feedback.Finding{
			warning(feedback.InlineSpanInverted, "%s: line range start %d is after its end %d", label, *start, end),
		}
	}
	return feedback.Span{First: *start, Last: end}, nil
}

// reports whether every line the anchor names is a line number, 1 or above.
func linesInRange(inline wireInline) bool {
	for _, line := range []*int{inline.From, inline.To, inline.StartFrom, inline.StartTo} {
		if line != nil && *line < 1 {
			return false
		}
	}
	return true
}

// refines the pre-image and post-image line fields onto the four position cases.
// Only `From` and `To` select the case. A start field widens its span and is
// ignored when its line is absent.
//
// An anchor naming line 0 or below addresses no line, so it is made whole file and
// reported. Its thread stays on its file.
func positionOfInline(inline wireInline, commentID int) (feedback.Position, []feedback.Finding) {
	label := fmt.Sprintf("comment %d on %s", commentID, inline.Path)
	if !linesInRange(inline) {
		return feedback.Position{Kind: feedback.PositionWholeFile}, []feedback.Finding{
			warning(feedback.InlineLineOutOfRange, "%s: names a line below 1; treated as the whole file", label),
		}
	}
	switch {
	case inline.From == nil && inline.To == nil:
		return feedback.Position{Kind: feedback.PositionWholeFile}, nil
	case inline.To == nil:
		lines, findings := span(inline.StartFrom, *inline.From, label)
		return feedback.Position{Kind: feedback.PositionOriginal, Lines: lines}, findings
	case inline.From == nil:
		lines, findings := span(inline.StartTo, *inline.To, label)
		return feedback.Position{Kind: feedback.PositionUpdated, Lines: lines}, findings
	default:
		original, first := span(inline.StartFrom, *inline.From, label)
		updated, second := span(inline.StartTo, *inline.To, label)
		return feedback.Position{Kind: feedback.PositionUnchanged, OriginalLines: original, UpdatedLines: updated},
			append(first, second...)
	}
}

// an anchor with no path cannot address a file, so its thread becomes review-level
// discussion and the loss is reported.
func locationOfInline(inline *wireInline, commentID int) (feedback.Location, []feedback.Finding) {
	if inline == nil {
		return feedback.Location{Kind: feedback.LocationPullRequest}, nil
	}
	if strings.Trim(inline.Path, blank) == "" {
		return feedback.Location{Kind: feedback.LocationPullRequest}, []feedback.Finding{
			failure(feedback.InlineWithoutPath, "comment %d has an inline anchor with no path", commentID),
		}
	}
	position, findings := positionOfInline(*inline, commentID)
	return feedback.Location{Kind: feedback.LocationFileRevision, Path: inline.Path, Position: position}, findings
}

func resolution(wire wireResolution, commentID int) (feedback.Resolution, []feedback.Finding) {
	switch wire.State {
	case resolutionEmpty:
		return feedback.Resolution{State: feedback.ResolutionResolvedWithoutInfo}, []feedback.Finding{
			warning(feedback.ResolutionDetailMissing,
				"comment %d is resolved but the payload carried no detail; the request omitted fields=+values.resolution.*",
				commentID),
		}
	case resolutionDetailed:
		by, findings := author(wire.User)
		at, more := parseTime(wire.CreatedOn, fmt.Sprintf("comment %d resolution", commentID))
		return feedback.Resolution{State: feedback.ResolutionResolved, By: &by, At: timestamp(at)},
			append(findings, more...)
	default:
		return feedback.Resolution{State: feedback.ResolutionUnresolved}, nil
	}
}

// a comment built with its parsed creation time, which orders replies.
type built struct {
	comment feedback.Comment
	created time.Time
}

func before(a built, b built) bool {
	if !a.created.Equal(b.created) {
		return a.created.Before(b.created)
	}
	return a.comment.ID < b.comment.ID
}

func shell(wire wireComment, replies []feedback.Comment) (built, []feedback.Finding) {
	findings := make([]feedback.Finding, 0)
	commentAuthor, more := author(wire.User)
	findings = append(findings, more...)
	commentBody, more := body(wire.ID, wire.Deleted, wire.BodyRaw)
	findings = append(findings, more...)
	created, more := parseTime(wire.CreatedOn, fmt.Sprintf("comment %d created_on", wire.ID))
	findings = append(findings, more...)
	updated, more := optionalTime(wire.UpdatedOn, fmt.Sprintf("comment %d updated_on", wire.ID))
	findings = append(findings, more...)
	return built{
		comment: feedback.Comment{
			ID:        wire.ID,
			Author:    commentAuthor,
			Body:      commentBody,
			CreatedAt: timestamp(created),
			UpdatedAt: updated,
			Pending:   wire.Pending,
			Replies:   replies,
		},
		created: created,
	}, findings
}

// the input indexed by identifier, with every identifier after its first occurrence
// dropped and reported: a forest keyed by identifier cannot hold both.
func index(comments []wireComment) (map[int]wireComment, []wireComment, []feedback.Finding) {
	byID := make(map[int]wireComment, len(comments))
	unique := make([]wireComment, 0, len(comments))
	findings := make([]feedback.Finding, 0)
	for _, comment := range comments {
		if _, seen := byID[comment.ID]; seen {
			findings = append(findings, failure(feedback.DuplicateCommentID,
				"comment identifier %d appears more than once; the later one is dropped", comment.ID))
			continue
		}
		byID[comment.ID] = comment
		unique = append(unique, comment)
	}
	return byID, unique, findings
}

// the forest builder. `visited` guards against a parent chain that loops: a comment
// already built is never descended into again, so building terminates on any input.
type forest struct {
	children map[int][]wireComment
	visited  map[int]bool
}

func (f *forest) build(wire wireComment) (built, []feedback.Finding) {
	f.visited[wire.ID] = true
	kids := make([]built, 0)
	findings := make([]feedback.Finding, 0)
	for _, child := range f.children[wire.ID] {
		if f.visited[child.ID] {
			continue
		}
		reply, more := f.build(child)
		kids = append(kids, reply)
		findings = append(findings, more...)
	}
	sort.SliceStable(kids, func(i, j int) bool { return before(kids[i], kids[j]) })
	replies := make([]feedback.Comment, 0, len(kids))
	for _, kid := range kids {
		replies = append(replies, kid.comment)
	}
	result, own := shell(wire, replies)
	return result, append(own, findings...)
}

// a reply's location comes from its thread root. One carrying a differing anchor of
// its own is reported.
func replyAnchorFindings(unique []wireComment, byID map[int]wireComment) []feedback.Finding {
	findings := make([]feedback.Finding, 0)
	for _, comment := range unique {
		if comment.ParentID == nil || comment.Inline == nil {
			continue
		}
		parent, found := byID[*comment.ParentID]
		if !found {
			continue
		}
		if parent.Inline == nil || parent.Inline.Path != comment.Inline.Path {
			findings = append(findings, warning(feedback.ReplyAnchorConflict,
				"reply %d anchors to %s but its parent does not", comment.ID, comment.Inline.Path))
		}
	}
	return findings
}

func resolutionOnReplyFindings(unique []wireComment) []feedback.Finding {
	findings := make([]feedback.Finding, 0)
	for _, comment := range unique {
		if comment.ParentID != nil && comment.Resolution.State != resolutionAbsent {
			findings = append(findings, warning(feedback.ResolutionOnReply,
				"comment %d is a reply yet carries a resolution", comment.ID))
		}
	}
	return findings
}

func chronologyFindings(unique []wireComment, byID map[int]wireComment) []feedback.Finding {
	findings := make([]feedback.Finding, 0)
	for _, comment := range unique {
		if comment.ParentID == nil {
			continue
		}
		parent, found := byID[*comment.ParentID]
		if !found {
			continue
		}
		child, childErr := feedback.ParseTimestamp(comment.CreatedOn)
		parentTime, parentErr := feedback.ParseTimestamp(parent.CreatedOn)
		if childErr == nil && parentErr == nil && child.Before(parentTime) {
			findings = append(findings, warning(feedback.ReplyPrecedesParent,
				"reply %d was created before its parent %d", comment.ID, *comment.ParentID))
		}
	}
	return findings
}

// a thread with the creation time of its root, which orders threads.
type builtThread struct {
	thread feedback.Thread
	root   built
}

func threadsOf(comments []wireComment) ([]feedback.Thread, []feedback.Finding) {
	byID, unique, duplicates := index(comments)

	children := make(map[int][]wireComment)
	roots := make([]wireComment, 0)
	orphans := make([]wireComment, 0)
	for _, comment := range unique {
		switch {
		case comment.ParentID == nil:
			roots = append(roots, comment)
		case !present(byID, *comment.ParentID):
			orphans = append(orphans, comment)
		default:
			children[*comment.ParentID] = append(children[*comment.ParentID], comment)
		}
	}
	orphanFindings := make([]feedback.Finding, 0, len(orphans))
	for _, orphan := range orphans {
		orphanFindings = append(orphanFindings, warning(feedback.OrphanReply,
			"reply %d names parent %d, which is not in the fetched set; promoted to a thread root",
			orphan.ID, *orphan.ParentID))
	}

	f := &forest{children: children, visited: make(map[int]bool)}
	threads := make([]builtThread, 0)
	buildFindings := make([]feedback.Finding, 0)
	add := func(wire wireComment) {
		root, findings := f.build(wire)
		location, more := locationOfInline(wire.Inline, wire.ID)
		findings = append(findings, more...)
		state, more := resolution(wire.Resolution, wire.ID)
		findings = append(findings, more...)
		threads = append(threads, builtThread{
			thread: feedback.Thread{Location: location, Resolution: state, Root: root.comment},
			root:   root,
		})
		buildFindings = append(buildFindings, findings...)
	}
	for _, wire := range append(roots, orphans...) {
		add(wire)
	}

	// a comment no root reaches is in a parent cycle: its parent is present, so it
	// is no orphan, yet no root leads to it. Promoting the lowest such identifier and
	// repeating breaks the cycle and keeps every comment exactly once.
	for {
		unreached := lowestUnreached(unique, f.visited)
		if unreached == nil {
			break
		}
		buildFindings = append(buildFindings, failure(feedback.ReplyCycle,
			"comment %d is part of a reply cycle; promoted to a thread root to break it", unreached.ID))
		add(*unreached)
	}

	sort.SliceStable(threads, func(i, j int) bool { return before(threads[i].root, threads[j].root) })
	result := make([]feedback.Thread, 0, len(threads))
	for _, thread := range threads {
		result = append(result, thread.thread)
	}

	findings := make([]feedback.Finding, 0)
	findings = append(findings, duplicates...)
	findings = append(findings, orphanFindings...)
	findings = append(findings, buildFindings...)
	findings = append(findings, replyAnchorFindings(unique, byID)...)
	findings = append(findings, resolutionOnReplyFindings(unique)...)
	findings = append(findings, chronologyFindings(unique, byID)...)
	return result, findings
}

func present(byID map[int]wireComment, id int) bool {
	_, found := byID[id]
	return found
}

// the comment with the lowest identifier not yet built, or nil.
func lowestUnreached(unique []wireComment, visited map[int]bool) *wireComment {
	var lowest *wireComment
	for i := range unique {
		if visited[unique[i].ID] {
			continue
		}
		if lowest == nil || unique[i].ID < lowest.ID {
			lowest = &unique[i]
		}
	}
	return lowest
}

// the known pull request states, lowercased as the document writes them. An
// unknown state is kept verbatim and reported.
var states = map[string]string{
	"OPEN": "open", "MERGED": "merged", "DECLINED": "declined", "SUPERSEDED": "superseded",
}

var roles = map[string]string{"REVIEWER": "reviewer", "PARTICIPANT": "participant"}

func pullRequest(wire wirePullRequest) (feedback.PullRequest, []feedback.Finding) {
	findings := make([]feedback.Finding, 0)
	prAuthor, more := author(wire.Author)
	findings = append(findings, more...)

	state, known := states[wire.State]
	if !known {
		state = wire.State
		findings = append(findings, warning(feedback.UnknownPullRequestState,
			"pull request state %q is outside the known set", wire.State))
	}

	created, more := parseTime(wire.CreatedOn, fmt.Sprintf("pull request %d created_on", wire.ID))
	findings = append(findings, more...)
	updated, more := optionalTime(wire.UpdatedOn, fmt.Sprintf("pull request %d updated_on", wire.ID))
	findings = append(findings, more...)

	participants := make([]feedback.Participant, 0, len(wire.Participants))
	for _, participant := range wire.Participants {
		user, more := author(participant.User)
		findings = append(findings, more...)
		role, known := roles[participant.Role]
		if !known {
			role = participant.Role
		}
		participants = append(participants, feedback.Participant{User: user, Role: role, Approved: participant.Approved})
	}

	var description *string
	if strings.Trim(wire.Description, blank) != "" {
		description = &wire.Description
	}

	return feedback.PullRequest{
		ID:                   wire.ID,
		Title:                wire.Title,
		Description:          description,
		State:                state,
		Draft:                wire.Draft,
		Author:               prAuthor,
		CreatedAt:            timestamp(created),
		UpdatedAt:            updated,
		Source:               feedback.Revision{Branch: wire.Source.Branch, Commit: wire.Source.Commit},
		Destination:          feedback.Revision{Branch: wire.Destination.Branch, Commit: wire.Destination.Commit},
		Participants:         participants,
		ReportedCommentCount: wire.CommentCount,
		ReportedTaskCount:    wire.TaskCount,
	}, findings
}

// normalises a pull request and its comments onto a document, with the findings met
// along the way in report order.
//
// For input whose comment identifiers are distinct, every input comment appears
// exactly once in the result, and identical input yields an identical document.
func normalise(wirePR wirePullRequest, comments []wireComment) (feedback.Document, []feedback.Finding) {
	pr, prFindings := pullRequest(wirePR)
	threads, threadFindings := threadsOf(comments)

	doc := feedback.Document{PullRequest: pr, Discussion: []feedback.Thread{}, FileFeedback: []feedback.Thread{}}
	for _, thread := range threads {
		if thread.Location.Kind == feedback.LocationPullRequest {
			doc.Discussion = append(doc.Discussion, thread)
		} else {
			doc.FileFeedback = append(doc.FileFeedback, thread)
		}
	}
	return doc, feedback.Sort(append(prFindings, threadFindings...))
}

// normalises recorded payloads: a pull request, and its comments as either a page
// envelope or a bare array. Issues no request. Returns an error only when a payload
// does not decode.
func NormalisePayloads(prJSON []byte, commentsJSON []byte) (feedback.Document, []feedback.Finding, error) {
	pr, err := decodePullRequest(prJSON)
	if err != nil {
		return feedback.Document{}, nil, &Error{Kind: Undecodable, Detail: err.Error()}
	}
	comments, err := decodeComments(commentsJSON)
	if err != nil {
		return feedback.Document{}, nil, &Error{Kind: Undecodable, Detail: err.Error()}
	}
	doc, findings := normalise(pr, comments)
	return doc, findings, nil
}

// retrieves a pull request and every comment on it, and normalises them. Nothing is
// returned unless both retrievals succeed.
func Fetch(ctx context.Context, client Client, ref PullRequestRef) (feedback.Document, []feedback.Finding, error) {
	pr, err := client.PullRequest(ctx, ref)
	if err != nil {
		return feedback.Document{}, nil, err
	}
	comments, err := client.Comments(ctx, ref)
	if err != nil {
		return feedback.Document{}, nil, err
	}
	doc, findings := normalise(pr, comments)
	return doc, findings, nil
}
