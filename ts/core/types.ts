// the wire types crossing the Wails bridge, mirroring the Go `model` package
// and the backend diff types. The bridge is JSON in both directions, so these
// are the decoded shapes of what the bound `App` methods return and accept.
// They are re-declared here (the Go side is the source of truth); task 3.3 pins
// these shapes against fixtures so drift fails a test rather than at runtime.

export type CommentStatus = "active" | "resolved" | "ignored";

// one entry in a comment's anchor history, mirroring the Go `Anchor`. `blob` is
// the git blob SHA of the file content this anchor was computed against;
// `context` is the captured window of raw line contents centred on the anchored
// line. An anchor with an empty/absent `context` is adrift: the content could
// not be located against that blob, so its `line_number` is not meaningful.
export interface Anchor {
  blob: string;
  line_number: number;
  offset?: number;
  context?: string[];
}

// a review note against a line of the diff. A reply is a `Comment` whose
// `parent_id` is the id of the comment it answers; a root comment has an empty
// `parent_id`. Replies form a flat thread under their root and carry no
// meaningful status.
//
// A comment is anchored through `anchors`, an ordered history (one entry per
// distinct blob it has been reconciled against). The current placement and
// whether the comment is outdated are derived from the most-recent anchor (see
// `currentLineNumber`/`isOutdated` in `core/comments`), never stored. Replies
// and review-level comments carry no anchors.
export interface Comment {
  id: string;
  parent_id?: string;
  author: string;
  content: string;
  status: CommentStatus;
  anchors?: Anchor[];
}

// the line classification, matching the Go `LineType` iota: 0 context, 1 added,
// 2 removed.
export const LineContext = 0;
export const LineAdded = 1;
export const LineRemoved = 2;
export type LineType =
  | typeof LineContext
  | typeof LineAdded
  | typeof LineRemoved;

export interface DiffLine {
  Type: LineType;
  Content: string;
  OldLineNo: number;
  NewLineNo: number;
}

export interface DiffHunk {
  OldStart: number;
  OldLines: number;
  NewStart: number;
  NewLines: number;
  Lines: DiffLine[];
}

export interface DiffFile {
  Path: string;
  Hunks: DiffHunk[];
  Binary: boolean;
}

// a file path paired with its comments, as returned by `GetCommentedFiles`.
export interface CommentedFile {
  path: string;
  comments: Comment[];
}

// the result of a comment mutation: enough to patch the one affected thread and
// the file's status pill without a full re-render. `file_path` is empty for a
// review-level comment; `line_number` is the affected thread's root line, -1 for
// a review-level thread.
export interface CommentMutationResult {
  file_path: string;
  line_number: number;
  comments: Comment[];
  file_status: FileStatus;
}

// the aggregate review status of a file, matching the Go `FileCommentStatus`
// return values.
export type FileStatus = "active" | "resolved" | "ignored" | "none";

// the working-tree status returned by `GetWorkingTreeStatus`.
export interface WorkingTreeStatus {
  modified: string[];
  deleted: string[];
  dirty_files: Record<string, boolean>;
}

// the metadata-only diff file shape returned by `GetDiffFiles` (no hunks).
export interface DiffFileMeta {
  Path: string;
  Binary: boolean;
}

// the result shape of `GetFileLines`: a range of context lines plus the file's
// total new-side line count.
export interface FileLinesResult {
  Lines: DiffLine[];
  TotalNew: number;
}
