package main

import (
	"code-review/model"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	flag "github.com/spf13/pflag"
)

// the agent-facing command-line interface to an in-progress review. It resolves
// the same state file the GUI uses from the current directory and branch, and
// performs read-only git operations only: its sole write target is the state
// file. The GUI is unaffected — `main` routes here only when invoked with a
// recognised subcommand.

// reviewContext is everything a CLI command needs: the resolved review, the
// path it was loaded from (and is saved back to), and the git user to attribute
// new replies and comments to. It is assembled once per invocation by
// `resolveReview`.
type reviewContext struct {
	review    *model.Review
	statePath string
	userName  string
}

// commentView is the purpose-built JSON shape for a comment. It exposes the
// agent-relevant fields and deliberately omits the internal anchor/blob history:
// `Line` is the comment's current placement and `Outdated` flags an adrift
// anchor, both derived from the model rather than surfaced raw. `File` is empty
// for a review-level comment. `Replies` is present only on a `show`.
type commentView struct {
	ID       string      `json:"id"`
	File     string      `json:"file"`
	Line     int         `json:"line"`
	Outdated bool        `json:"outdated"`
	Author   string      `json:"author"`
	Content  string      `json:"content"`
	Status   string      `json:"status"`
	Replies  []replyView `json:"replies,omitempty"`
}

// replyView is the purpose-built JSON shape for a reply: a comment with a
// parent, carrying no line or context of its own.
type replyView struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Content string `json:"content"`
}

// statusView is the purpose-built JSON shape for `status`: the branches, the
// counts of root comments by status, and the marked-file count.
type statusView struct {
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Active       int    `json:"active"`
	Resolved     int    `json:"resolved"`
	Ignored      int    `json:"ignored"`
	MarkedFiles  int    `json:"marked_files"`
}

// flatten a model comment to its view, attaching the owning file (empty for a
// review-level comment). Replies are not attached here; callers that need a
// thread build it explicitly.
func toCommentView(filePath string, comment *model.Comment) commentView {
	return commentView{
		ID:       comment.ID,
		File:     filePath,
		Line:     comment.CurrentLineNumber(),
		Outdated: comment.IsOutdated(),
		Author:   comment.Author,
		Content:  comment.Content,
		Status:   string(comment.Status),
	}
}

// the flat reply thread of `rootID` within `comments`, in stored order. Replies
// are flat (a reply to a reply re-roots), so every reply whose `ParentID`
// equals the root belongs to the thread.
func threadReplies(comments []*model.Comment, rootID string) []replyView {
	replies := make([]replyView, 0)
	for _, comment := range comments {
		if comment.ParentID == rootID {
			replies = append(replies, replyView{ID: comment.ID, Author: comment.Author, Content: comment.Content})
		}
	}
	return replies
}

// the comments belonging to a surface, paired with the surface's file path. The
// returned path is empty for the review-level surface. Used to scan every
// surface uniformly.
type surface struct {
	filePath string
	comments []*model.Comment
}

// every comment surface of the review: one per file plus the review-level
// surface (empty path). The order is files-in-diff-order then review-level,
// matching how the GUI groups feedback.
func surfaces(review *model.Review) []surface {
	out := make([]surface, 0, len(review.Files)+1)
	for _, file := range review.Files {
		out = append(out, surface{filePath: file.FilePath, comments: file.Comments})
	}
	out = append(out, surface{filePath: "", comments: review.Comments})
	return out
}

// locate a comment by id across every file and the review-level comments,
// returning the comment, the path of its owning surface (empty for
// review-level), the surface's comment slice (for thread/root lookups), and
// whether it was found.
func findComment(review *model.Review, commentID string) (*model.Comment, string, []*model.Comment, bool) {
	for _, s := range surfaces(review) {
		for _, comment := range s.comments {
			if comment.ID == commentID {
				return comment, s.filePath, s.comments, true
			}
		}
	}
	return nil, "", nil, false
}

// require exactly `n` positional arguments, returning a usage-style error
// naming the command otherwise.
func requireArgs(command string, args []string, n int, usage string) error {
	if len(args) != n {
		return fmt.Errorf("usage: code-review %s %s", command, usage)
	}
	return nil
}

// write `value` as indented JSON to `out`, followed by a newline.
func emitJSON(out io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

// resolve a root comment for a status change: the comment must exist and be a
// root (replies carry no meaningful status). Returns a clear error otherwise,
// so the caller leaves state untouched.
func rootComment(review *model.Review, commentID string) (*model.Comment, error) {
	comment, _, _, found := findComment(review, commentID)
	if !found {
		return nil, fmt.Errorf("comment not found: %s", commentID)
	}
	if comment.ParentID != "" {
		return nil, fmt.Errorf("%s is a reply, not a root comment", commentID)
	}
	return comment, nil
}

// list — the active root comments across files and the review level, as a JSON
// array. Empty array when none. Read-only.
func cmdList(ctx *reviewContext, out io.Writer, args []string) error {
	views := make([]commentView, 0)
	for _, s := range surfaces(ctx.review) {
		for _, comment := range s.comments {
			if comment.ParentID == "" && comment.Status == model.CommentStatusActive {
				views = append(views, toCommentView(s.filePath, comment))
			}
		}
	}
	return emitJSON(out, views)
}

// show — one root comment with its full reply thread and current placement.
// Read-only; an unknown id is an error.
func cmdShow(ctx *reviewContext, out io.Writer, args []string) error {
	if err := requireArgs("show", args, 1, "<id>"); err != nil {
		return err
	}
	comment, filePath, comments, found := findComment(ctx.review, args[0])
	if !found {
		return fmt.Errorf("comment not found: %s", args[0])
	}
	view := toCommentView(filePath, comment)
	view.Replies = threadReplies(comments, comment.ID)
	return emitJSON(out, view)
}

// status — the review summary: branches, root-comment counts by status, and the
// marked-file count. Read-only.
func cmdStatus(ctx *reviewContext, out io.Writer, args []string) error {
	view := statusView{
		SourceBranch: ctx.review.SourceBranch,
		TargetBranch: ctx.review.TargetBranch,
		MarkedFiles:  len(ctx.review.MarkedFiles),
	}
	for _, s := range surfaces(ctx.review) {
		for _, comment := range s.comments {
			if comment.ParentID != "" {
				continue
			}
			switch comment.Status {
			case model.CommentStatusActive:
				view.Active++
			case model.CommentStatusResolved:
				view.Resolved++
			case model.CommentStatusIgnored:
				view.Ignored++
			}
		}
	}
	return emitJSON(out, view)
}

// resolve — set a root comment resolved and persist. Errors (and leaves state
// untouched) on a reply target or unknown id.
func cmdResolve(ctx *reviewContext, out io.Writer, args []string) error {
	if err := requireArgs("resolve", args, 1, "<id>"); err != nil {
		return err
	}
	comment, err := rootComment(ctx.review, args[0])
	if err != nil {
		return err
	}
	comment.Resolve()
	return saveReview(ctx, out, fmt.Sprintf("resolved %s", comment.ID))
}

// reactivate — set a resolved root comment back to active and persist. Same
// error handling as resolve.
func cmdReactivate(ctx *reviewContext, out io.Writer, args []string) error {
	if err := requireArgs("reactivate", args, 1, "<id>"); err != nil {
		return err
	}
	comment, err := rootComment(ctx.review, args[0])
	if err != nil {
		return err
	}
	comment.Reactivate()
	return saveReview(ctx, out, fmt.Sprintf("reactivated %s", comment.ID))
}

// reply — append a reply to a comment's thread, attributed to the git user, and
// persist. Routes to the file or review-level surface that owns the comment. The
// parent's status is unchanged. Errors (no write) on an unknown id.
func cmdReply(ctx *reviewContext, out io.Writer, args []string) error {
	if err := requireArgs("reply", args, 2, "<id> <text>"); err != nil {
		return err
	}
	commentID, content := args[0], args[1]
	_, filePath, _, found := findComment(ctx.review, commentID)
	if !found {
		return fmt.Errorf("comment not found: %s", commentID)
	}
	if filePath == "" {
		ctx.review.AddReply(commentID, content, ctx.userName)
	} else {
		ctx.review.GetFileDiff(filePath).AddReply(commentID, content, ctx.userName)
	}
	return saveReview(ctx, out, fmt.Sprintf("replied to %s", commentID))
}

// comment — add a review-level (top-level, unattached) comment as the git user,
// and persist. Errors (no write) on empty text.
func cmdComment(ctx *reviewContext, out io.Writer, args []string) error {
	if err := requireArgs("comment", args, 1, "<text>"); err != nil {
		return err
	}
	if strings.TrimSpace(args[0]) == "" {
		return fmt.Errorf("comment text must not be empty")
	}
	added := ctx.review.AddComment(args[0], ctx.userName)
	return saveReview(ctx, out, fmt.Sprintf("added review comment %s", added.ID))
}

// unmark — remove a file from the marked-files set and persist. Unmarking an
// unmarked file succeeds with the set unchanged.
func cmdUnmark(ctx *reviewContext, out io.Writer, args []string) error {
	if err := requireArgs("unmark", args, 1, "<file>"); err != nil {
		return err
	}
	ctx.review.UnmarkFile(args[0])
	return saveReview(ctx, out, fmt.Sprintf("unmarked %s", args[0]))
}

// instructions — print the embedded agent contract verbatim. Read-only and does
// not resolve a review, so it works outside a repository.
func cmdInstructions(out io.Writer, args []string) error {
	_, err := fmt.Fprint(out, instructionsText)
	return err
}

// start — create the review state file for the current directory and branch if
// it does not already exist. This is the only command that creates a review;
// every other command requires one to already exist. It writes only the state
// file (read-only on the repository) and is a no-op report if a review is
// already present, never overwriting existing feedback. The state seeded here is
// empty: it carries no diff or file list yet.
func cmdStart(target *reviewTarget, out io.Writer, args []string) error {
	if err := requireArgs("start", args, 0, ""); err != nil {
		return err
	}

	if _, err := os.Stat(target.statePath); err == nil {
		_, err := fmt.Fprintf(out, "review already exists for branch %q at %s\n", target.sourceBranch, target.statePath)
		return err
	}

	// ensure the data directory exists before the first write, as the GUI does
	// in startup; `SaveReview` opens the file but does not create its parent.
	if err := os.MkdirAll(filepath.Dir(target.statePath), 0o755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	review := model.NewReview(target.repoPath, target.sourceBranch, target.defaultBranch)
	if err := SaveReview(target.statePath, review); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "started review for %q against %q at %s\n", target.sourceBranch, target.defaultBranch, target.statePath)
	return err
}

// persist the review to its state path and print a short confirmation. The
// state file is the only write target of any mutating command.
func saveReview(ctx *reviewContext, out io.Writer, message string) error {
	if err := SaveReview(ctx.statePath, ctx.review); err != nil {
		return err
	}
	_, err := fmt.Fprintln(out, message)
	return err
}

// the resolution a command requires before it runs. `instructions` needs
// nothing (it works outside a repository); `start` needs the review target (the
// repo/branch derivation) so it can create the state file; every other command
// needs a loaded review.
type needs int

const (
	needsNothing needs = iota
	needsTarget
	needsReview
)

// command is one CLI subcommand: its handlers (only one of which is set,
// matching `needs`), a one-line usage summary for the help listing, and the
// resolution it requires. The split-typed handlers keep each command's signature
// honest about what it receives rather than passing a half-built context.
type command struct {
	needs         needs
	summary       string
	runReview     func(ctx *reviewContext, out io.Writer, args []string) error
	runTarget     func(target *reviewTarget, out io.Writer, args []string) error
	runStandalone func(out io.Writer, args []string) error
}

// the CLI command table — the single definition of the command set, used both
// to dispatch and to build the help listing.
var commands = map[string]command{
	"start":        {needs: needsTarget, summary: "create the review for the current branch", runTarget: cmdStart},
	"list":         {needs: needsReview, summary: "list the active comments needing attention", runReview: cmdList},
	"show":         {needs: needsReview, summary: "show <id> — one comment with its reply thread", runReview: cmdShow},
	"status":       {needs: needsReview, summary: "summarise the review (branches, counts, marks)", runReview: cmdStatus},
	"resolve":      {needs: needsReview, summary: "resolve <id> — mark a comment addressed", runReview: cmdResolve},
	"reactivate":   {needs: needsReview, summary: "reactivate <id> — set a resolved comment active", runReview: cmdReactivate},
	"reply":        {needs: needsReview, summary: "reply <id> <text> — reply to a comment", runReview: cmdReply},
	"comment":      {needs: needsReview, summary: "comment <text> — add a review-level comment", runReview: cmdComment},
	"unmark":       {needs: needsReview, summary: "unmark <file> — drop a file's reviewed mark", runReview: cmdUnmark},
	"instructions": {needs: needsNothing, summary: "print the agent contract", runStandalone: cmdInstructions},
}

// reviewTarget identifies the review for the current directory and branch
// without loading it: the repository, the source and default (target) branches,
// the git user, and the state path the review lives at. It is the common
// derivation shared by `start` (which may create the file) and `resolveReview`
// (which requires it to exist).
type reviewTarget struct {
	repoPath      string
	sourceBranch  string
	defaultBranch string
	userName      string
	statePath     string
}

// derive the review target for the current directory and branch, mirroring the
// GUI's startup derivation: git root, current and default branch, the XDG data
// dir, and the resulting state path. Performs read-only git operations only and
// does not touch the state file. Returns a clear error when there is no
// repository.
func resolveTarget() (*reviewTarget, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	repoPath, err := GetGitRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("not a git repository: run code-review inside the repository under review")
	}

	userName, err := GetUserName(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get git user name: %w", err)
	}

	sourceBranch, err := GetCurrentBranch(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	defaultBranch, err := GetDefaultBranch(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch: %w", err)
	}

	statePath := GetReviewStatePath(GetXDGDataDir(), repoPath, sourceBranch, defaultBranch)
	return &reviewTarget{
		repoPath:      repoPath,
		sourceBranch:  sourceBranch,
		defaultBranch: defaultBranch,
		userName:      userName,
		statePath:     statePath,
	}, nil
}

// resolve and load the review for the current directory and branch. Builds the
// target, requires the state file to exist (never creating it), and loads it.
// Returns a clear error when there is no repository, or no review for the
// current branch.
func resolveReview() (*reviewContext, error) {
	target, err := resolveTarget()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(target.statePath); err != nil {
		return nil, fmt.Errorf("no review found for branch %q (run `code-review start` to begin one)", target.sourceBranch)
	}

	review, err := LoadReview(target.statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load review: %w", err)
	}

	return &reviewContext{review: review, statePath: target.statePath, userName: target.userName}, nil
}

// the sorted command names, for a stable help listing.
func commandNames() []string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// write the usage listing — the command set and a one-line summary each — to
// `out`, built from the command table so there is one definition.
func writeUsage(out io.Writer) {
	fmt.Fprintln(out, "code-review — review changes between the current branch and the default branch.")
	fmt.Fprintln(out, "\nRun with no arguments to open the GUI, or use a command:")
	for _, name := range commandNames() {
		fmt.Fprintf(out, "  %-12s %s\n", name, commands[name].summary)
	}
}

// run the CLI for `args` (the arguments after the program name), returning a
// process exit code. The first argument selects the command; `-h`/`--help`
// print usage. An unknown command is an error. `instructions` runs without a
// review; every other command resolves one first.
func runCLI(args []string, out io.Writer, errOut io.Writer) int {
	name := args[0]
	if name == "-h" || name == "--help" {
		writeUsage(out)
		return 0
	}

	cmd, ok := commands[name]
	if !ok {
		fmt.Fprintf(errOut, "unknown command: %s\n\n", name)
		writeUsage(errOut)
		return 2
	}

	// per-command flag parsing: flags after the command name are parsed
	// POSIX-style, leaving the positional arguments for the handler.
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}

	// dispatch by the resolution each command needs, passing it exactly the
	// context its handler expects.
	var runErr error
	switch cmd.needs {
	case needsNothing:
		runErr = cmd.runStandalone(out, flags.Args())
	case needsTarget:
		target, err := resolveTarget()
		if err != nil {
			fmt.Fprintf(errOut, "%v\n", err)
			return 1
		}
		runErr = cmd.runTarget(target, out, flags.Args())
	case needsReview:
		ctx, err := resolveReview()
		if err != nil {
			fmt.Fprintf(errOut, "%v\n", err)
			return 1
		}
		runErr = cmd.runReview(ctx, out, flags.Args())
	}

	if runErr != nil {
		fmt.Fprintf(errOut, "%v\n", runErr)
		return 1
	}
	return 0
}

// report whether `args` (the arguments after the program name) name a CLI
// invocation rather than a GUI launch. A bare invocation, or a leading flag
// (e.g. the GUI's `--version`) other than the help flag, is a GUI launch; any
// other leading word is a CLI invocation — a recognised command runs, an
// unrecognised one errors with usage rather than silently opening the GUI.
func isCLIInvocation(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "-h" || args[0] == "--help" {
		return true
	}
	// a leading flag belongs to the GUI's own flag parsing (`--version`).
	if strings.HasPrefix(args[0], "-") {
		return false
	}
	return true
}
