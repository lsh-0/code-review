// the render layer's mutable view state. It is deliberately a single shared
// object rather than scattered module globals, so the render modules read and
// update one place. This is view state
// (what is selected, what is cached for rendering) — not domain logic, which
// lives in the pure core.

import type { Comment, DiffFile } from "../core/types.ts";
import { primaryFile } from "../core/selection.ts";

export interface ViewState {
  // the ordered set of files shown in the diff pane. A plain click selects one;
  // a ctrl/cmd-click toggles a file in or out. Empty means no file is shown (the
  // overview, or the not-yet-loaded initial state).
  selectedFiles: string[];

  // the primary (anchor) file: the last-toggled selected path, or "" when the
  // selection is empty. A computed accessor over `selectedFiles`, kept for the
  // file-list highlight fallback and the refresh path that need one "current"
  // file. Assigning it replaces the whole selection (empty string clears it).
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

  // the current file-list filter term; an empty string shows every file.
  fileFilter: string;

  // the key of the active file-list grouper; "none" renders a flat list.
  fileGrouping: string;

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
  selectedFiles: [],
  // `currentFile` is derived from `selectedFiles`: reading it returns the primary
  // (last-toggled) path; assigning it replaces the selection. This keeps the many
  // single-file read/write sites working unchanged while `selectedFiles` is the
  // source of truth.
  get currentFile(): string {
    return primaryFile(this.selectedFiles);
  },
  set currentFile(filePath: string) {
    this.selectedFiles = filePath === "" ? [] : [filePath];
  },
  currentUser: "",
  overviewActive: false,
  diffFiles: [],
  commentsCache: new Map(),
  markedFiles: new Set(),
  fileFilter: "",
  fileGrouping: "none",
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
