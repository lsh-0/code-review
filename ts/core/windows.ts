// the window model: a file's diff renders as a list of windows — contiguous,
// disjoint ranges of new-side line numbers — separated by hidden gaps. Default
// windows come from the hunks plus an island per comment outside every hunk;
// growth extends a window into its gap; normalisation keeps the list sorted,
// disjoint, and merged wherever a gap shrinks below the threshold. All pure
// functions over plain data: the render layer walks the results to build DOM,
// and re-rendering from the same state reproduces the same view.

import {
  type DiffHunk,
  type DiffLine,
  LineContext,
  LineRemoved,
} from "./types.ts";

// a contiguous inclusive range of new-side line numbers.
export interface Window {
  start: number;
  end: number;
}

// the number of lines one activation of a grow affordance reveals.
export const growStep = 20;

// the diff is generated with git's default of 3 trailing context lines, so a
// last hunk carrying fewer than this many reached end-of-file.
export const diffContextSize = 3;

// two windows merge when the hidden gap between them holds fewer than this many
// lines: a 1- or 2-line sliver of hidden context is noise, so it is revealed
// and the windows join. The converging-growth merge (gap reaches zero) is the
// degenerate case. Equal to the ordinary diff context by design: a comment
// island within a context-width of a hunk reads as part of it.
export const windowMergeThreshold = diffContextSize;

// the new-side range a hunk's lines occupy. A pathological hunk with no
// new-side lines (a no-context pure deletion) is given a single-line span at
// its position so its removed lines still have a window to render in.
export function hunkSpan(hunk: DiffHunk): Window {
  return {
    start: hunk.NewStart,
    end: hunk.NewStart + Math.max(hunk.NewLines, 1) - 1,
  };
}

// canonicalise a window list: clamp to the file's bounds (`totalLines` 0 =
// unknown, no upper clamp), drop empty windows, sort, and merge any pair whose
// hidden gap is below `windowMergeThreshold`. Every operation that changes
// windows runs its result through here, so the invariants — sorted, disjoint,
// gaps at least the threshold — hold everywhere by construction.
export function normaliseWindows(
  windows: Window[],
  totalLines: number,
): Window[] {
  const clamped = windows
    .map((w) => ({
      start: Math.max(w.start, 1),
      end: totalLines > 0 ? Math.min(w.end, totalLines) : w.end,
    }))
    .filter((w) => w.end >= w.start)
    .sort((a, b) => a.start - b.start);

  const merged: Window[] = [];
  for (const w of clamped) {
    const prev = merged[merged.length - 1];
    if (prev && w.start - prev.end - 1 < windowMergeThreshold) {
      prev.end = Math.max(prev.end, w.end);
      continue;
    }
    merged.push({ ...w });
  }
  return merged;
}

// whether `line` falls inside any of `windows`.
export function windowsContain(windows: Window[], line: number): boolean {
  return windows.some((w) => line >= w.start && line <= w.end);
}

// the default windows for a file: one per hunk (its natural span) plus an
// island per commented line outside every hunk span — the commented line with
// the ordinary diff context each side — normalised, so islands near a hunk
// merge into it. `commentLines` holds the placed (non-outdated) comment lines;
// line 0 entries (top-level comments) produce no window. This is what a file
// shows on open and what refresh collapses back to.
export function defaultWindows(
  hunks: DiffHunk[],
  commentLines: number[],
  totalLines: number,
): Window[] {
  const spans = hunks.map(hunkSpan);
  const islands = commentLines
    .filter((line) => line > 0 && !windowsContain(spans, line))
    .map((line) => ({
      start: line - diffContextSize,
      end: line + diffContextSize,
    }));
  return normaliseWindows([...spans, ...islands], totalLines);
}

export const growUp = "up";
export const growDown = "down";
export type GrowDirection = typeof growUp | typeof growDown;

// grow the window at `index` by `step` lines in a direction, then normalise —
// so a growth that closes (or nearly closes) a gap merges the windows, and one
// that hits a file bound clamps. Growth never shrinks any window.
export function growWindow(
  windows: Window[],
  index: number,
  direction: GrowDirection,
  step: number,
  totalLines: number,
): Window[] {
  const grown = windows.map((w, i) => {
    if (i !== index) {
      return w;
    }
    if (direction === growUp) {
      return { start: w.start - step, end: w.end };
    }
    return { start: w.start, end: w.end + step };
  });
  return normaliseWindows(grown, totalLines);
}

// one stretch of a window's render plan: either a whole hunk (rendered from
// its own lines, removed lines interleaved) or a run of unchanged context
// lines (rendered from the fetched file body). `oldOffset` maps a context
// line's new number to its old number (old = new + oldOffset), constant across
// an unchanged run.
export type WindowSegment =
  | { kind: "hunk"; hunkIndex: number }
  | { kind: "context"; start: number; end: number; oldOffset: number };

// the old-line offset for unchanged lines *before* hunk `i` (equivalently,
// after hunk `i-1`): old = new + offset.
function offsetBeforeHunk(hunk: DiffHunk): number {
  return hunk.OldStart - hunk.NewStart;
}

// the old-line offset for unchanged lines *after* a hunk.
function offsetAfterHunk(hunk: DiffHunk): number {
  return (hunk.OldStart + hunk.OldLines) - (hunk.NewStart + hunk.NewLines);
}

// the old-line offset for a context run starting at `line`, derived from the
// nearest hunk: after the last hunk that ends before `line`, or (for a run
// above every hunk) before the first hunk that starts after it. A file with no
// hunks in reach has identical numbering (offset 0).
function contextOffsetAt(hunks: DiffHunk[], line: number): number {
  let after: number | null = null;
  for (const hunk of hunks) {
    if (hunkSpan(hunk).end < line) {
      after = offsetAfterHunk(hunk);
    }
  }
  if (after !== null) {
    return after;
  }
  for (const hunk of hunks) {
    if (hunkSpan(hunk).start > line) {
      return offsetBeforeHunk(hunk);
    }
  }
  return 0;
}

// a window's render plan: its range decomposed into hunk segments and context
// runs, in file order. Hunks are wholly inside or wholly outside a window by
// the normalisation invariant (a window can never stop partway through a
// hunk), so each overlapping hunk contributes one whole segment.
export function windowPlan(
  window: Window,
  hunks: DiffHunk[],
): WindowSegment[] {
  const segments: WindowSegment[] = [];
  let pos = window.start;

  for (let i = 0; i < hunks.length; i++) {
    const span = hunkSpan(hunks[i]);
    if (span.end < window.start || span.start > window.end) {
      continue;
    }
    if (span.start > pos) {
      segments.push({
        kind: "context",
        start: pos,
        end: span.start - 1,
        oldOffset: offsetBeforeHunk(hunks[i]),
      });
    }
    segments.push({ kind: "hunk", hunkIndex: i });
    pos = span.end + 1;
  }

  if (pos <= window.end) {
    segments.push({
      kind: "context",
      start: pos,
      end: window.end,
      oldOffset: contextOffsetAt(hunks, pos),
    });
  }
  return segments;
}

// the context ranges of a window plan whose lines are not yet in the fetched
// cache — what the render layer must request before those lines can render.
// Returned as maximal contiguous sub-ranges of missing lines.
export function missingRanges(
  segments: WindowSegment[],
  fetched: (line: number) => boolean,
): { start: number; end: number; oldOffset: number }[] {
  const missing: { start: number; end: number; oldOffset: number }[] = [];
  for (const segment of segments) {
    if (segment.kind !== "context") {
      continue;
    }
    let runStart = 0;
    for (let line = segment.start; line <= segment.end + 1; line++) {
      const isMissing = line <= segment.end && !fetched(line);
      if (isMissing && runStart === 0) {
        runStart = line;
      }
      if (!isMissing && runStart !== 0) {
        missing.push({
          start: runStart,
          end: line - 1,
          oldOffset: segment.oldOffset,
        });
        runStart = 0;
      }
    }
  }
  return missing;
}

// whether a hunk reaches the end of the file on the new side, so the downward
// grow control can be disabled up front with nothing below to reveal.
//
// The signal is the trailing context run: git emits up to `diffContextSize`
// unchanged lines after a hunk's last change, so a shorter run means the file
// ended. This only holds when the hunk's last line is a real new-side line
// (context or added). A hunk ending in removed lines says nothing about the new
// side, so it makes no end-of-file claim.
export function hunkReachedEOF(lines: DiffLine[]): boolean {
  if (lines.length === 0) {
    return false;
  }
  if (lines[lines.length - 1].Type === LineRemoved) {
    return false;
  }

  let trailing = 0;
  for (let i = lines.length - 1; i >= 0; i--) {
    if (lines[i].Type !== LineContext) {
      break;
    }
    trailing++;
  }
  return trailing < diffContextSize;
}
