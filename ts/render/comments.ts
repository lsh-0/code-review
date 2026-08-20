// renders comment threads (root comments and their nested replies) with full
// edit/delete/reply/status actions. The threading and authorship *decisions*
// come from the pure core (`getReplies`, `threadAuthors`, `authorLabel`); this
// module builds the DOM from those results and wires the actions straight to
// the modal/mutation handlers in `modals.ts`.

import type { Comment } from "../core/types.ts";
import {
  authorLabel,
  getReplies,
  outdatedComments,
  threadAuthors,
  topLevelComments,
} from "../core/comments.ts";
import { el } from "../dom.ts";
import { commentsFor, state } from "./state.ts";
import {
  deleteComment,
  ignoreComment,
  reactivateComment,
  resolveComment,
  showEditCommentModal,
  showFileCommentModal,
  showReplyModal,
} from "./modals.ts";

// build a comment thread element for the given root comments. Replies nest
// within each root via the shared rendering.
export function createCommentThread(
  filePath: string,
  comments: Comment[],
): HTMLElement {
  const thread = el("div", { classes: ["comment-thread"] });
  for (const comment of comments) {
    const multipleAuthors =
      threadAuthors(commentsFor(filePath), comment).size > 1;
    thread.appendChild(
      createCommentElement(filePath, comment, multipleAuthors),
    );
  }
  return thread;
}

// build the top-of-file block for a file's top-level (anchorless) comments: an
// optional "Add comment" button followed by each top-level comment's thread.
// It renders at the very top of the file view, above the untethered block and
// the code.
//
// `withAddButton` (single-file view) always shows an "Add comment" control, so
// the block is present even with no comments yet; without it (overview,
// read-only) the block renders only when there are top-level comments,
// returning null otherwise.
export function topLevelCommentsBlock(
  filePath: string,
  comments: Comment[],
  withAddButton: boolean,
): HTMLElement | null {
  const topLevel = topLevelComments(comments);
  if (!withAddButton && topLevel.length === 0) {
    return null;
  }

  const container = el("div", { classes: ["top-level-comments"] });
  container.setAttribute("data-toplevel", filePath);

  if (withAddButton) {
    container.appendChild(
      el("button", {
        classes: ["top-level-add-comment"],
        text: "Add comment",
        attrs: { title: "Add a comment about this file" },
        onClick: () => showFileCommentModal(filePath),
      }),
    );
  }

  if (topLevel.length > 0) {
    container.appendChild(createCommentThread(filePath, topLevel));
  }

  return container;
}

// build the untethered block: every outdated root comment, plus any live
// anchored comment whose line cannot be shown in a window (`unplaceable` —
// e.g. a line beyond the end of the file). Returns null when there are none.
// Each item shows the comment's last captured context as a read-only
// pseudo-hunk with a warning border (no line numbers, no grow affordance)
// followed by the comment thread. An unplaceable live comment keeps its anchor
// untouched: rendering here is a fallback, not a state change. Intended to
// render at the top of the file view, above the windows.
export function outdatedCommentsBlock(
  filePath: string,
  comments: Comment[],
  unplaceable: Comment[] = [],
): HTMLElement | null {
  const untethered = [...outdatedComments(comments), ...unplaceable];
  if (untethered.length === 0) {
    return null;
  }

  const container = el("div", { classes: ["outdated-comments"] });
  container.setAttribute("data-outdated", filePath);
  for (const comment of untethered) {
    container.appendChild(outdatedCommentItem(filePath, comment));
  }
  return container;
}

// one untethered comment: its captured context as a read-only pseudo-hunk
// followed by the comment thread, wrapped so the warning styling applies to
// both.
function outdatedCommentItem(
  filePath: string,
  comment: Comment,
): HTMLElement {
  const item = el("div", { classes: ["outdated-comment"] });
  item.setAttribute("data-comment", comment.id);

  const context = comment.context ?? [];
  if (context.length > 0) {
    const hunk = el("div", { classes: ["outdated-hunk"] });
    for (const line of context) {
      hunk.appendChild(
        el("div", { classes: ["diff-line", "outdated-line"], text: line }),
      );
    }
    item.appendChild(hunk);
  }

  item.appendChild(createCommentThread(filePath, [comment]));
  return item;
}

// a line-anchored comment thread carrying a `data-line` attribute, so tests
// and navigation can find the thread for a line.
export function lineCommentThread(
  filePath: string,
  lineNo: number,
  comments: Comment[],
): HTMLElement {
  const thread = createCommentThread(filePath, comments);
  thread.setAttribute("data-line", String(lineNo));
  return thread;
}

// render a comment (root or reply). A reply (parent_id set) is styled as a
// reply, omits the status badge, and offers no status actions — only root
// comments carry a meaningful status. Replies nest after the content.
function createCommentElement(
  filePath: string,
  comment: Comment,
  multipleAuthors: boolean,
): HTMLElement {
  const isReply = !!comment.parent_id;
  const commentID = comment.id;

  const elem = el("div", {
    classes: isReply ? ["comment", "comment-reply"] : ["comment"],
  });

  const header = el("div", { classes: ["comment-header"] });
  if (!isReply) {
    header.appendChild(
      el("span", {
        classes: ["comment-status", comment.status],
        text: comment.status,
      }),
    );
  }
  const label = authorLabel(comment, multipleAuthors, state.currentUser);
  if (label) {
    header.appendChild(
      el("span", { classes: ["comment-author"], text: ` ${label}` }),
    );
  }
  elem.appendChild(header);

  elem.appendChild(
    el("div", { classes: ["comment-content"], text: comment.content }),
  );

  if (!isReply) {
    const replies = getReplies(commentsFor(filePath), commentID);
    if (replies.length > 0) {
      const repliesElem = el("div", { classes: ["comment-replies"] });
      for (const reply of replies) {
        repliesElem.appendChild(
          createCommentElement(filePath, reply, multipleAuthors),
        );
      }
      elem.appendChild(repliesElem);
    }
  }

  const actionsElem = el("div", { classes: ["comment-actions"] });

  // Reply applies only to root comments: the thread is flat. Replies get
  // edit/delete parity but no status actions.
  if (!isReply) {
    actionsElem.appendChild(
      el("button", {
        text: "Reply",
        onClick: () => showReplyModal(filePath, commentID),
      }),
    );
  }
  actionsElem.appendChild(
    el("button", {
      text: "Edit",
      onClick: () => showEditCommentModal(filePath, commentID, comment.content),
    }),
  );
  if (!isReply) {
    if (comment.status === "active") {
      actionsElem.appendChild(
        el("button", {
          text: "Resolve",
          onClick: () => void resolveComment(filePath, commentID),
        }),
      );
      actionsElem.appendChild(
        el("button", {
          text: "Ignore",
          onClick: () => void ignoreComment(filePath, commentID),
        }),
      );
    } else {
      actionsElem.appendChild(
        el("button", {
          text: "Reactivate",
          onClick: () => void reactivateComment(filePath, commentID),
        }),
      );
    }
  }
  actionsElem.appendChild(
    el("button", {
      classes: ["delete-btn"],
      text: "Delete",
      onClick: () => void deleteComment(filePath, commentID),
    }),
  );

  elem.appendChild(actionsElem);
  return elem;
}
