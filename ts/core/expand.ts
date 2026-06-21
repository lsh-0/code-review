// the context-expansion geometry. These are pure functions over the expand
// state; the render layer owns the DOM rows and frontier mutation, calling these
// for every arithmetic decision. Keeping them pure is what makes the
// converging-affordance logic unit-testable without a DOM.

import { type DiffLine, LineContext, LineRemoved } from "./types.ts";

export const expandUp = "up";
export const expandDown = "down";
export type ExpandDirection = typeof expandUp | typeof expandDown;

export const expandStep = 20;

// the diff is generated with git's default of 3 trailing context lines, so a
// last hunk carrying fewer than this many reached end-of-file.
export const diffContextSize = 3;

// the mutable per-affordance state: the next hidden line adjacent to the hunk
// (`frontier`) and the furthest hidden line in the gap (`boundary`, 0 = unknown
// trailing end). A between-hunk gap's two converging affordances are linked by
// `sibling`: each one's live frontier is the other's true boundary, so they
// meet in the middle without overshooting. Mirrors the Go `expandState`, minus
// the DOM handles, which stay in the render layer.
export interface ExpandState {
  frontier: number;
  boundary: number;
  sibling?: ExpandState;
}

// the furthest hidden line this affordance may reveal toward. For a linked
// between-hunk affordance that is the sibling's current frontier (they converge
// on each other); otherwise the fixed gap boundary.
export function effectiveBoundary(state: ExpandState): number {
  if (state.sibling) {
    return state.sibling.frontier;
  }
  return state.boundary;
}

// the inclusive new-line range to request for the next step in a direction,
// clamped to the remaining gap.
export function stepRange(
  direction: ExpandDirection,
  state: ExpandState,
): { startNew: number; endNew: number } {
  const boundary = effectiveBoundary(state);
  if (direction === expandUp) {
    const endNew = state.frontier;
    let startNew = endNew - expandStep + 1;
    if (startNew < boundary) {
      startNew = boundary;
    }
    return { startNew, endNew };
  }
  const startNew = state.frontier;
  let endNew = startNew + expandStep - 1;
  if (boundary > 0 && endNew > boundary) {
    endNew = boundary;
  }
  return { startNew, endNew };
}

// move the frontier toward the gap boundary by the number of revealed lines,
// and learn the trailing boundary from the reported total on first fetch.
// Returns the new state rather than mutating, so callers compose it.
export function advanceState(
  direction: ExpandDirection,
  state: ExpandState,
  revealed: number,
  totalNew: number,
): ExpandState {
  if (direction === expandUp) {
    return { ...state, frontier: state.frontier - revealed };
  }
  const frontier = state.frontier + revealed;
  const boundary = state.boundary === 0 ? totalNew : state.boundary;
  return { ...state, frontier, boundary };
}

// whether a gap has been fully revealed in the given direction. For a linked
// between-hunk gap the boundary is the sibling's frontier, so the gap closes
// when the two frontiers cross.
export function gapExhausted(
  direction: ExpandDirection,
  state: ExpandState,
): boolean {
  const boundary = effectiveBoundary(state);
  if (direction === expandUp) {
    return state.frontier < boundary;
  }
  return boundary > 0 && state.frontier > boundary;
}

// whether a hunk reaches the end of the file on the new side, so the downward
// expand control can be disabled up front with nothing below to reveal.
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
