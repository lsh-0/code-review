// pure context-extraction and overview-window logic: line-context capture and
// the visibility computation for commented hunks. Both are pure functions of the
// diff data and the commented lines; the render layer consumes their results to
// build DOM.

import type { DiffFile, DiffHunk } from "./types.ts";

// how many context lines to capture on each side of a commented line. A wider
// window than a single neighbour gives the re-anchoring heuristic more material
// to match on as the file changes. The captured window is the anchored line
// plus up to this many lines either side, drawn from the same hunk.
//
// MUST stay equal to the backend's `captureRadius` (backend/model/reanchor.go):
// this captures the window at creation and the backend re-captures it on
// re-anchor, so both must produce the same window size. The two constants are
// independent; keep them in lockstep.
export const captureContextRadius = 3;

// the captured context window for a comment anchored at `lineNumber` in `file`:
// the new-side line contents from `captureContextRadius` lines before it through
// `captureContextRadius` lines after it, within the same hunk, paired with
// `offset` — the index of the anchored line within the returned window. `offset`
// equals `captureContextRadius` for an interior line and is smaller when the
// window is clipped by the start of the hunk. Returns an empty window and offset
// 0 when the file or line is not found. The backend reconciler anchors to the
// line at `offset`, so the window need not be symmetric.
export function getLineContext(
  file: DiffFile | undefined,
  lineNumber: number,
): { context: string[]; offset: number } {
  if (!file) {
    return { context: [], offset: 0 };
  }

  for (const hunk of file.Hunks) {
    for (let i = 0; i < hunk.Lines.length; i++) {
      if (hunk.Lines[i].NewLineNo !== lineNumber) {
        continue;
      }
      const lo = Math.max(0, i - captureContextRadius);
      const hi = Math.min(hunk.Lines.length - 1, i + captureContextRadius);
      return {
        context: hunk.Lines.slice(lo, hi + 1).map((l) => l.Content),
        offset: i - lo,
      };
    }
  }

  return { context: [], offset: 0 };
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
