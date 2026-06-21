// the diff context-expansion affordances: the "expand N lines" rows above and
// below hunks, the converging between-hunk merge, and the splice of revealed
// lines. All the *arithmetic* (next range, advance, exhaustion) is the pure core
// in ../core/expand.ts; this module owns only the DOM rows and the frontier
// mutation, and a render-side struct that pairs the pure `ExpandState` with its
// DOM handles.

import type { DiffHunk, DiffLine } from "../core/types.ts";
import {
  advanceState,
  type ExpandDirection,
  expandDown,
  type ExpandState,
  expandStep,
  expandUp,
  gapExhausted,
  stepRange,
} from "../core/expand.ts";
import { getFileLines } from "../client.ts";
import { el } from "../dom.ts";
import { type CommentActions } from "./comments.ts";
import { commentThreadOrNil, createDiffLine } from "./diff.ts";

// the render-side affordance: the pure expand state plus its DOM handles.
// Keeping the pure state in its own field means the core arithmetic never sees
// the DOM. Two converging between-hunk affordances
// link via `sibling` (their pure states are linked too); the upward one records
// the lower hunk and header so the merge can hide the join from either side.
interface Affordance {
  pure: ExpandState;
  direction: ExpandDirection;
  row: HTMLElement;
  oldOffset: number;
  sibling?: Affordance;
  lowerHunk?: HTMLElement;
  lowerHeader?: HTMLElement;
}

// callbacks the splice needs to rebuild a revealed line and its thread.
export interface ExpandContext {
  filePath: string;
  actions: CommentActions;
  onAddComment: (filePath: string, lineNo: number) => void;
}

// add an expansion affordance row to a hunk element and return it. `direction`
// is expandUp (reveal before the hunk, inserted above the header) or expandDown
// (reveal after, appended). `frontier` is the next hidden new-line adjacent to
// the hunk; `boundary` is the furthest hidden new line (0 for a trailing gap
// whose end is unknown until fetched). `startDisabled` forces a disabled row
// up front (a known end-of-file gap).
export function addExpandAffordance(
  hunkElem: HTMLElement,
  ctx: ExpandContext,
  direction: ExpandDirection,
  hunk: DiffHunk,
  frontier: number,
  boundary: number,
  startDisabled: boolean,
): Affordance {
  const arrow = direction === expandDown ? "↓" : "↑";
  const row = el("div", {
    classes: ["expand-row"],
    text: `${arrow} expand ${expandStep} lines`,
  });

  const oldOffset = hunk.OldStart - hunk.NewStart;
  const aff: Affordance = {
    pure: { frontier, boundary },
    direction,
    row,
    oldOffset,
  };

  // an affordance with no hidden lines in its gap is disabled from the start. A
  // downward affordance whose boundary is unknown (0) starts enabled unless the
  // caller already knows the hunk reached end-of-file.
  if (
    (direction === expandUp && frontier < boundary) ||
    (direction === expandDown && boundary > 0 && frontier > boundary) ||
    startDisabled
  ) {
    disableExpandRow(row);
  }

  row.addEventListener("click", () => {
    if (row.classList.contains("disabled")) {
      return;
    }
    void expandGap(hunkElem, ctx, aff);
  });

  hunkElem.appendChild(row);
  return aff;
}

// link two converging between-hunk affordances as siblings, so each one's live
// frontier is the other's true boundary and they meet in the middle. The upward
// affordance carries the lower hunk and header for the merge.
export function linkSiblings(
  up: Affordance,
  down: Affordance,
  lowerHunk: HTMLElement,
  lowerHeader: HTMLElement,
): void {
  up.sibling = down;
  down.sibling = up;
  up.pure.sibling = down.pure;
  down.pure.sibling = up.pure;
  up.lowerHunk = lowerHunk;
  up.lowerHeader = lowerHeader;
}

// request the next step of context lines and splice them into the gap, then
// re-evaluate whether the affordance should remain.
async function expandGap(
  hunkElem: HTMLElement,
  ctx: ExpandContext,
  aff: Affordance,
): Promise<void> {
  const { startNew, endNew } = stepRange(aff.direction, aff.pure);

  let result;
  try {
    result = await getFileLines(ctx.filePath, startNew, endNew, aff.oldOffset);
  } catch {
    // a failed range request (binary, missing path) disables the affordance
    // rather than leaving a control that never works.
    disableExpandRow(aff.row);
    return;
  }

  spliceRevealed(hunkElem, ctx, aff, result.Lines);
  aff.pure = advanceState(
    aff.direction,
    aff.pure,
    result.Lines.length,
    result.TotalNew,
  );

  if (gapExhausted(aff.direction, aff.pure)) {
    if (aff.sibling) {
      mergeBetweenHunks(aff);
    } else {
      // top-/end-of-file gap exhausted: keep the row in place but disabled so
      // navigation does not shift.
      disableExpandRow(aff.row);
    }
  }
}

// insert revealed lines (with their comment threads) into the gap in file
// order. A fragment preserves ascending order in one insertion. Upward: the row
// sits above the header, so blocks are inserted just after the row. Downward:
// inserted before the row, which stays at the bottom.
function spliceRevealed(
  hunkElem: HTMLElement,
  ctx: ExpandContext,
  aff: Affordance,
  lines: DiffLine[],
): void {
  const fragment = document.createDocumentFragment();
  for (const line of lines) {
    fragment.appendChild(createDiffLine(line, ctx.filePath, ctx.onAddComment));
    const thread = commentThreadOrNil(
      ctx.filePath,
      line.NewLineNo,
      ctx.actions,
    );
    if (thread) {
      fragment.appendChild(thread);
    }
  }

  if (aff.direction === expandUp) {
    hunkElem.insertBefore(fragment, aff.row.nextSibling);
    return;
  }
  hunkElem.insertBefore(fragment, aff.row);
}

// merge the two hunks bracketing a fully revealed between-hunk gap into one
// continuous block. `aff` is either converging affordance; the upward one
// carries the lower hunk and header. Both rows are removed, the lower header
// hidden, and the join classes drop the visual separation.
function mergeBetweenHunks(aff: Affordance): void {
  let up: Affordance | undefined = aff;
  if (!up.lowerHunk) {
    up = aff.sibling;
  }
  if (!up || !up.lowerHunk || !up.lowerHeader) {
    return;
  }

  aff.row.remove();
  aff.sibling?.row.remove();

  up.lowerHeader.classList.add("joined-hidden");
  up.lowerHunk.classList.add("joined-above");
  const prev = up.lowerHunk.previousElementSibling;
  if (prev) {
    prev.classList.add("joined-below");
  }
}

// mark an expansion row disabled: it stays in place for stable navigation but
// is struck through and ignores clicks.
export function disableExpandRow(row: HTMLElement): void {
  row.classList.add("disabled");
}
