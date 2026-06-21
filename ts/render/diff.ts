// renders diff lines and embeds comment threads at their anchored lines. The
// syntax-highlighting wiring lives here: a line's code goes through the
// highlighter at the render boundary, and the result is injected as innerHTML on
// the content node, with the plain-text fallback already escaped.

import { type DiffLine, LineAdded, LineRemoved } from "../core/types.ts";
import { getCommentsForLine } from "../core/comments.ts";
import { el } from "../dom.ts";
import { type CommentActions, lineCommentThread } from "./comments.ts";
import { highlightLine, languageForPath } from "./highlight.ts";
import { commentsFor } from "./state.ts";

// build a single diff-line row: the old/new line-number gutter (new number
// clickable to add a comment) and the highlighted content. A new-side line
// carries a `data-line` attribute so incremental comment updates can find it.
export function createDiffLine(
  line: DiffLine,
  filePath: string,
  onAddComment: (filePath: string, lineNo: number) => void,
): HTMLElement {
  const classes = ["diff-line"];
  if (line.Type === LineAdded) {
    classes.push("added");
  } else if (line.Type === LineRemoved) {
    classes.push("removed");
  }
  const lineElem = el("div", { classes });
  if (line.NewLineNo > 0) {
    lineElem.setAttribute("data-line", String(line.NewLineNo));
  }

  const numbers = el("div", { classes: ["line-numbers"] });

  // line numbers render via CSS (.line-number::before { content: attr(data-num) })
  // so they are never part of the selectable/copyable text when dragging.
  const oldNum = el("div", { classes: ["line-number"] });
  if (line.OldLineNo > 0) {
    oldNum.setAttribute("data-num", String(line.OldLineNo));
  }
  numbers.appendChild(oldNum);

  const newNum = el("div", { classes: ["line-number"] });
  if (line.NewLineNo > 0) {
    newNum.setAttribute("data-num", String(line.NewLineNo));
    newNum.classList.add("clickable");
    const lineNo = line.NewLineNo;
    newNum.addEventListener("click", () => onAddComment(filePath, lineNo));
  }
  numbers.appendChild(newNum);
  lineElem.appendChild(numbers);

  // the content node receives highlighted HTML (the highlighter escapes its
  // input, and the plain-text fallback escapes too), so innerHTML is safe here.
  const content = el("div", { classes: ["line-content"] });
  content.innerHTML = highlightLine(line.Content, languageForPath(filePath));
  lineElem.appendChild(content);

  return lineElem;
}

// append a comment thread for `lineNo` to `parent` when comments exist there,
// and mark `lineElem` (the row the thread sits under) as commented so it reads
// as one block with the thread.
export function appendCommentThread(
  parent: Element,
  lineElem: HTMLElement,
  filePath: string,
  lineNo: number,
  actions: CommentActions,
): void {
  const comments = getCommentsForLine(commentsFor(filePath), lineNo);
  if (comments.length > 0) {
    lineElem.classList.add("commented");
    parent.appendChild(lineCommentThread(filePath, lineNo, comments, actions));
  }
}

// a comment thread for a line, or null when there are none. Used when splicing
// revealed context lines so each carries its thread if it has one.
export function commentThreadOrNil(
  filePath: string,
  lineNo: number,
  actions: CommentActions,
): HTMLElement | null {
  const comments = getCommentsForLine(commentsFor(filePath), lineNo);
  if (comments.length === 0) {
    return null;
  }
  return lineCommentThread(filePath, lineNo, comments, actions);
}

// whether a new-side line carries a comment, for the overview's
// comment-only-hunk filtering.
export function hasCommentsForLine(filePath: string, lineNo: number): boolean {
  return getCommentsForLine(commentsFor(filePath), lineNo).length > 0;
}
