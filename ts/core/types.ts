// the wire types crossing the Wails bridge, mirroring the Go `model` package
// and the backend diff types. The bridge is JSON in both directions, so these
// are the decoded shapes of what the bound `App` methods return and accept.
// They are re-declared here (the Go side is the source of truth); task 3.3 pins
// these shapes against fixtures so drift fails a test rather than at runtime.

export type CommentStatus = "active" | "resolved" | "ignored";

// an external review imported into this one, mirroring the Go `Source`. `id` is
// what an imported comment's `origin` names, and `label` is what the visibility
// toggle displays.
export interface Source {
  id: string;
  label: string;
  pull_request: number;
  title?: string;
  source_branch: string;
  target_branch: string;
  imported_at: string;
}

// the source id standing for the review's own locally written feedback. It is not
// a real entry in the review's sources: it is the key local comments group under
// so the visibility toggles treat local feedback as one source among the imports
// rather than as a special case.
export const localSourceID = "local";

// a review note against a line of the diff, in the derived shape the backend
// sends (the Go `guiComment`). A reply is a `Comment` whose `parent_id` is the
// id of the comment it answers; a root comment has an empty `parent_id`.
// Replies form a flat thread under their root and carry no meaningful status.
//
// Placement is backend-derived: `line` is the current new-side line number (0
// for replies, top-level file comments, review-level comments, and outdated
// comments), `outdated` reports whether the comment can no longer be placed,
// and `context` is the last good captured window — the content rendered for an
// untethered comment. Anchor histories never cross the bridge.
export interface Comment {
  id: string;
  parent_id?: string;
  author: string;
  content: string;
  status: CommentStatus;
  line: number;
  outdated: boolean;
  context?: string[];
  // the external review this comment was imported from, absent when written
  // locally. Mirrors the Go `guiComment.Origin`.
  origin?: string;
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
  // whether git detected this file as a rename. A pure rename (100% similarity)
  // carries no hunks; a rename with edits carries both `Renamed` and its hunks.
  Renamed: boolean;
  // the pre-rename path, set only when `Renamed` is true; empty otherwise.
  OldPath: string;
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
  Renamed: boolean;
  // the pre-rename path, set only when `Renamed` is true; empty otherwise.
  OldPath: string;
}

// the result shape of `GetFileLines`: a range of context lines plus the file's
// total new-side line count.
export interface FileLinesResult {
  Lines: DiffLine[];
  TotalNew: number;
}
