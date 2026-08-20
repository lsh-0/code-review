// the render layer's mutable view state. It is deliberately a single shared
// object rather than scattered module globals, so the render modules read and
// update one place. This is view state (what is selected, what is cached for
// rendering) — not domain logic, which lives in the pure core. Everything on
// screen is derivable from here: the DOM is disposable output, and re-rendering
// a file's region from this state reproduces the same view.

import type { Comment, DiffFile, WorkingTreeStatus } from "../core/types.ts";
import type { Window } from "../core/windows.ts";
import { primaryFile } from "../core/selection.ts";

// one file's window state: which ranges of the file are shown, the fetched
// head-revision line contents backing the context runs, and the file's total
// line count (0 until the first fetch reports it). Kept across file switches —
// grown windows survive navigating away and back — and dropped wholesale on a
// diff refresh, which collapses every file to its default windows.
export interface FileViewState {
  windows: Window[];
  // fetched line contents by new-side line number; sparse.
  fileLines: Map<number, string>;
  totalLines: number;
  // a line-range fetch failed for this file (binary, vanished path): stop
  // re-requesting rather than looping.
  fetchFailed: boolean;
  // a fetch pass is in flight, so a re-render must not start a second one.
  fetching: boolean;
}

export interface ViewState {
  // the ordered set of files shown in the diff pane. A plain click selects one;
  // a ctrl/cmd-click toggles a file in or out. Empty means no file is shown (the
  // overview, or the not-yet-loaded initial state).
  selectedFiles: string[];

  currentUser: string;
  overviewActive: boolean;

  // the diff files with their (lazily fetched) hunks, the in-memory copy the
  // render layer reads. Metadata arrives first via GetDiffFiles; each file's
  // hunks are filled in on first selection via GetFileDiff.
  diffFiles: DiffFile[];

  // per-file window state, keyed by path. Populated on a file's first render;
  // cleared by a refresh so windows collapse back to their defaults.
  fileViews: Map<string, FileViewState>;

  // comments keyed by file path; the empty-string key holds review-level
  // comments. Primed by the overview/file loads and patched by mutations.
  commentsCache: Map<string, Comment[]>;

  // the set of files the reviewer has marked done.
  markedFiles: Set<string>;

  // the current file-list filter term; an empty string shows every file.
  fileFilter: string;

  // the key of the active file-list grouper; "none" renders a flat list.
  fileGrouping: string;

  // the most recently fetched working-tree status, so the per-file dirty
  // check on selection reads it without a fresh git call. Refreshed on load,
  // on every full refresh, and by the worktree watcher; null until the first
  // fetch succeeds.
  workingTreeStatus: WorkingTreeStatus | null;

  // transient modal context: which file/line/comment the open modal acts on.
  modal: {
    file: string;
    lineNumber: number;
    commentID: string;
    replyID: string;
    reviewCommentMode: boolean;
    fileCommentMode: boolean;
  };

  zoomLevel: number;
}

export const state: ViewState = {
  selectedFiles: [],
  currentUser: "",
  overviewActive: false,
  diffFiles: [],
  fileViews: new Map(),
  commentsCache: new Map(),
  markedFiles: new Set(),
  fileFilter: "",
  fileGrouping: "none",
  workingTreeStatus: null,
  modal: {
    file: "",
    lineNumber: 0,
    commentID: "",
    replyID: "",
    reviewCommentMode: false,
    fileCommentMode: false,
  },
  zoomLevel: 1.0,
};

// the primary (anchor) file of the selection: the last-toggled selected path,
// or "" when the selection is empty. The file-list highlight fallback and the
// per-file banners need one "current" file.
export function currentFile(): string {
  return primaryFile(state.selectedFiles);
}

// the comments for a surface: a file's cached comments, or the review-level
// comments under the empty-path key. Returns an empty array when absent, so
// callers can read it directly.
export function commentsFor(filePath: string): Comment[] {
  return state.commentsCache.get(filePath) ?? [];
}

// the new-side lines of a file's placed comments: roots with a live line. The
// input to default-window computation — outdated and top-level comments place
// no window.
export function placedCommentLines(filePath: string): number[] {
  return commentsFor(filePath)
    .filter((c) => !c.parent_id && !c.outdated && c.line > 0)
    .map((c) => c.line);
}

// the in-memory diff file entry for a path, or undefined if absent.
export function diffFile(filePath: string): DiffFile | undefined {
  return state.diffFiles.find((f) => f.Path === filePath);
}

// whether a diff entry still needs its hunks fetched: a text file with no hunks
// has not been loaded (or was reset by a refresh); a binary file never has
// hunks. A pure-rename file also has no hunks, but its fetch is free (the
// backend serves it from the already-parsed diff, no git I/O), so it is not
// special-cased here.
export function diffFileNeedsFetch(file: DiffFile): boolean {
  return file.Hunks.length === 0 && !file.Binary;
}
