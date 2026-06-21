// pure context-extraction and overview-window logic: line-context capture and
// the visibility computation for commented hunks. Both are pure functions of the
// diff data and the commented lines; the render layer consumes their results to
// build DOM.

import type { DiffFile, DiffHunk } from "./types.ts";

// the surrounding context for a comment anchored at `lineNumber` in `file`: the
// content of the new-side line, the line before it, and the line after it,
// within the same hunk. Returns empty strings when the file or line is not
// found. Mirrors the Go `getLineContext`.
export function getLineContext(
  file: DiffFile | undefined,
  lineNumber: number,
): { before: string; line: string; after: string } {
  const empty = { before: "", line: "", after: "" };
  if (!file) {
    return empty;
  }

  for (const hunk of file.Hunks) {
    for (let i = 0; i < hunk.Lines.length; i++) {
      const line = hunk.Lines[i];
      if (line.NewLineNo === lineNumber) {
        return {
          before: i > 0 ? hunk.Lines[i - 1].Content : "",
          line: line.Content,
          after: i < hunk.Lines.length - 1 ? hunk.Lines[i + 1].Content : "",
        };
      }
    }
  }

  return empty;
}

// the number of context lines shown around a comment in the overview, so a
// comment on a large hunk shows a tight window rather than the whole hunk. The
// comment renders below its anchored line, so showing `overviewContextLines`
// below it and one fewer above puts an equal count of code on each side.
export const overviewContextLines = 3;

// whether any new-side line in `hunk` carries a comment, given a predicate that
// reports whether a new-line number has comments. Lets the overview render only
// the hunks that have feedback. Mirrors the Go `hunkHasComments`.
export function hunkHasComments(
  hunk: DiffHunk,
  hasCommentsForLine: (lineNo: number) => boolean,
): boolean {
  return hunk.Lines.some(
    (line) => line.NewLineNo > 0 && hasCommentsForLine(line.NewLineNo),
  );
}

// the per-line visibility mask for a commented hunk in the overview: a line is
// visible if it falls within the context window of any commented line. One
// fewer line above each commented line than below (the commented line counts
// toward the lines above the comment), so the comment block sits centred.
// Mirrors the visibility computation in the Go `appendCommentedHunkLines`.
export function overviewVisibleLines(
  hunk: DiffHunk,
  hasCommentsForLine: (lineNo: number) => boolean,
): boolean[] {
  const visible = new Array<boolean>(hunk.Lines.length).fill(false);

  for (let i = 0; i < hunk.Lines.length; i++) {
    const line = hunk.Lines[i];
    if (line.NewLineNo <= 0 || !hasCommentsForLine(line.NewLineNo)) {
      continue;
    }
    let lo = i - (overviewContextLines - 1);
    if (lo < 0) {
      lo = 0;
    }
    let hi = i + overviewContextLines;
    if (hi > hunk.Lines.length - 1) {
      hi = hunk.Lines.length - 1;
    }
    for (let j = lo; j <= hi; j++) {
      visible[j] = true;
    }
  }

  return visible;
}
