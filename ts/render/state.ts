// the render layer's mutable view state. It is deliberately a single shared
// object rather than scattered module globals, so the render modules read and
// update one place. This is view state
// (what is selected, what is cached for rendering) — not domain logic, which
// lives in the pure core.

import type { Comment, DiffFile } from "../core/types.ts";

export interface ViewState {
  currentFile: string;
  currentUser: string;
  overviewActive: boolean;

  // the diff files with their (lazily fetched) hunks, the in-memory copy the
  // render layer reads. Metadata arrives first via GetDiffFiles; each file's
  // hunks are filled in on first selection via GetFileDiff.
  diffFiles: DiffFile[];

  // comments keyed by file path; the empty-string key holds review-level
  // comments. Primed by the overview/file loads and patched by mutations.
  commentsCache: Map<string, Comment[]>;

  // the set of files the reviewer has marked done.
  markedFiles: Set<string>;

  // transient modal context: which file/line/comment the open modal acts on.
  modal: {
    file: string;
    lineNumber: number;
    commentID: string;
    replyID: string;
    reviewCommentMode: boolean;
  };

  zoomLevel: number;
}

export const state: ViewState = {
  currentFile: "",
  currentUser: "",
  overviewActive: false,
  diffFiles: [],
  commentsCache: new Map(),
  markedFiles: new Set(),
  modal: {
    file: "",
    lineNumber: 0,
    commentID: "",
    replyID: "",
    reviewCommentMode: false,
  },
  zoomLevel: 1.0,
};

// the comments for a surface: a file's cached comments, or the review-level
// comments under the empty-path key. Returns an empty array when absent, so
// callers can read it directly.
export function commentsFor(filePath: string): Comment[] {
  return state.commentsCache.get(filePath) ?? [];
}

// the in-memory diff file entry for a path, or undefined if absent.
export function diffFile(filePath: string): DiffFile | undefined {
  return state.diffFiles.find((f) => f.Path === filePath);
}

// whether a diff entry still needs its hunks fetched: a text file with no hunks
// has not been loaded (or was reset by a refresh); a binary file never has
// hunks.
export function diffFileNeedsFetch(file: DiffFile): boolean {
  return file.Hunks.length === 0 && !file.Binary;
}
