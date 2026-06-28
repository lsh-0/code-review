// renders a file's hunks into a parent element: each hunk's header, its diff
// lines with embedded comment threads, and the inter-hunk / end-of-file expand
// affordances. The overview's comment-only branch lives in overview.ts (half 2).
// The between-hunk converging affordances are linked here via the pure state's
// sibling link.

import type { DiffHunk } from "../core/types.ts";
import { expandDown, expandUp, hunkReachedEOF } from "../core/expand.ts";
import { hunkHasComments, overviewVisibleLines } from "../core/overview.ts";
import { byId, clear, el } from "../dom.ts";
import { type CommentActions, outdatedCommentsBlock } from "./comments.ts";
import {
  appendCommentThread,
  createDiffLine,
  hasCommentsForLine,
} from "./diff.ts";
import {
  addExpandAffordance,
  type ExpandContext,
  linkSiblings,
} from "./expand.ts";
import { commentsFor, diffFile } from "./state.ts";

export interface DiffCallbacks {
  actions: CommentActions;
  onAddComment: (filePath: string, lineNo: number) => void;
}

// render a single file's diff into #diff-content.
export function renderDiff(filePath: string, cb: DiffCallbacks): void {
  const content = byId("diff-content");
  if (!content) {
    return;
  }
  clear(content);
  renderFileHunks(content, filePath, cb);
}

// render a file's hunks into `parent`. The single-file view (`overviewOnly`
// false) renders every hunk with the inter-hunk and end-of-file expand
// affordances. The overview (`overviewOnly` true) renders read-only and only
// the hunks carrying a comment, each trimmed to the context window around its
// commented lines. Binary files render a placeholder; a missing entry renders
// nothing.
export function renderFileHunks(
  parent: Element,
  filePath: string,
  cb: DiffCallbacks,
  overviewOnly = false,
): void {
  const file = diffFile(filePath);
  if (!file) {
    return;
  }

  if (file.Binary) {
    parent.appendChild(
      el("div", {
        classes: ["binary-placeholder"],
        text: "binary file, cannot diff",
      }),
    );
    return;
  }

  const ctx: ExpandContext = {
    filePath,
    actions: cb.actions,
    onAddComment: cb.onAddComment,
  };

  // outdated comments render untethered at the top of the single-file view: they
  // no longer anchor to any live hunk. The overview gathers per-hunk feedback and
  // omits them.
  if (!overviewOnly) {
    const outdated = outdatedCommentsBlock(
      filePath,
      commentsFor(filePath),
      cb.actions,
    );
    if (outdated) {
      parent.appendChild(outdated);
    }
  }

  let prevHunkElem: HTMLElement | null = null;
  for (let i = 0; i < file.Hunks.length; i++) {
    const hunk = file.Hunks[i];

    // in the overview, skip hunks with no comments: the page gathers feedback,
    // not the whole diff.
    if (
      overviewOnly &&
      !hunkHasComments(hunk, (n) => hasCommentsForLine(filePath, n))
    ) {
      continue;
    }

    const hunkElem = el("div", { classes: ["diff-hunk"] });

    const header = el("div", {
      classes: ["hunk-header"],
      text:
        `@@ -${hunk.OldStart},${hunk.OldLines} +${hunk.NewStart},${hunk.NewLines} @@`,
    });

    if (!overviewOnly) {
      // the gap above this hunk: between the previous hunk's last new line (or
      // the start of the file) and this hunk's first new line. The upward
      // affordance sits above the header.
      let prevEnd = 0;
      if (i > 0) {
        const prev = file.Hunks[i - 1];
        prevEnd = prev.NewStart + prev.NewLines - 1;
      }
      const betweenHunks = i > 0;
      const upAff = addExpandAffordance(
        hunkElem,
        ctx,
        expandUp,
        hunk,
        hunk.NewStart - 1,
        prevEnd + 1,
        false,
      );

      // a between-hunk gap also gets a downward affordance on the previous hunk,
      // linked to the upward one so they converge and merge in the middle.
      if (betweenHunks && prevHunkElem) {
        const prevHunk = file.Hunks[i - 1];
        const downAff = addExpandAffordance(
          prevHunkElem,
          ctx,
          expandDown,
          prevHunk,
          prevEnd + 1,
          hunk.NewStart - 1,
          false,
        );
        linkSiblings(upAff, downAff, hunkElem, header);
      }
    }

    hunkElem.appendChild(header);

    if (overviewOnly) {
      appendCommentedHunkLines(hunkElem, filePath, hunk, cb);
    } else {
      for (const line of hunk.Lines) {
        const lineElem = createDiffLine(line, filePath, cb.onAddComment);
        hunkElem.appendChild(lineElem);
        appendCommentThread(
          hunkElem,
          lineElem,
          filePath,
          line.NewLineNo,
          cb.actions,
        );
      }
    }

    // the gap below the last hunk extends to end-of-file. A last hunk whose
    // trailing context is shorter than the diff context size already reached
    // end-of-file, so the affordance is disabled up front; otherwise the first
    // fetch disables it if no further lines exist.
    if (!overviewOnly && i === file.Hunks.length - 1) {
      const lastEnd = hunk.NewStart + hunk.NewLines - 1;
      const atEOF = hunkReachedEOF(hunk.Lines);
      addExpandAffordance(
        hunkElem,
        ctx,
        expandDown,
        hunk,
        lastEnd + 1,
        0,
        atEOF,
      );
    }

    parent.appendChild(hunkElem);
    prevHunkElem = hunkElem;
  }
}

// append a commented hunk's lines to `parent`, but only those within the
// context window of a commented line (via the pure overviewVisibleLines), with
// each comment thread embedded after its line. Lines outside every window are
// dropped, so a comment in a large hunk shows just its neighbourhood.
function appendCommentedHunkLines(
  parent: HTMLElement,
  filePath: string,
  hunk: DiffHunk,
  cb: DiffCallbacks,
): void {
  const visible = overviewVisibleLines(
    hunk,
    (n) => hasCommentsForLine(filePath, n),
  );
  for (let i = 0; i < hunk.Lines.length; i++) {
    if (!visible[i]) {
      continue;
    }
    const line = hunk.Lines[i];
    const lineElem = createDiffLine(line, filePath, cb.onAddComment);
    parent.appendChild(lineElem);
    appendCommentThread(parent, lineElem, filePath, line.NewLineNo, cb.actions);
  }
}
