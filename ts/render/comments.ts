// renders comment threads (root comments and their nested replies) with full
// edit/delete/reply/status actions. The threading and authorship *decisions*
// come from the pure core (`getReplies`, `threadAuthors`, `authorLabel`); this
// module only builds the DOM from those results and wires the action callbacks.

import type { Comment } from "../core/types.ts";
import {
  authorLabel,
  getReplies,
  lastGoodContext,
  outdatedComments,
  threadAuthors,
} from "../core/comments.ts";
import { el } from "../dom.ts";
import { commentsFor, state } from "./state.ts";

// the action callbacks a comment thread needs. Supplied by the surface that
// renders the thread (file page or overview) so the same rendering routes
// actions to the right place via the file path it was built with.
export interface CommentActions {
  onReply: (filePath: string, commentID: string) => void;
  onEdit: (filePath: string, commentID: string, content: string) => void;
  onResolve: (filePath: string, commentID: string) => void;
  onIgnore: (filePath: string, commentID: string) => void;
  onReactivate: (filePath: string, commentID: string) => void;
  onDelete: (filePath: string, commentID: string) => void;
}

// build a comment thread element for the given root comments. Replies nest
// within each root via the shared rendering.
export function createCommentThread(
  filePath: string,
  comments: Comment[],
  actions: CommentActions,
): HTMLElement {
  const thread = el("div", { classes: ["comment-thread"] });
  for (const comment of comments) {
    const multipleAuthors =
      threadAuthors(commentsFor(filePath), comment).size > 1;
    thread.appendChild(
      createCommentElement(filePath, comment, multipleAuthors, actions),
    );
  }
  return thread;
}

// build the untethered block for every outdated root comment in `comments`, or
// null when none are outdated. Each block shows the comment's last captured
// context as a read-only pseudo-hunk with a warning border (no line numbers, no
// expand affordance) followed by the comment thread. The container carries
// `data-outdated` so an incremental update can find and replace it. Intended to
// render at the top of the file view, above the live hunks.
export function outdatedCommentsBlock(
  filePath: string,
  comments: Comment[],
  actions: CommentActions,
): HTMLElement | null {
  const outdated = outdatedComments(comments);
  if (outdated.length === 0) {
    return null;
  }

  const container = el("div", { classes: ["outdated-comments"] });
  container.setAttribute("data-outdated", filePath);
  for (const comment of outdated) {
    container.appendChild(outdatedCommentItem(filePath, comment, actions));
  }
  return container;
}

// one outdated comment: its captured context as a read-only pseudo-hunk followed
// by the comment thread, wrapped so the warning styling applies to both. The
// `data-comment` attribute lets an incremental update target this single item.
function outdatedCommentItem(
  filePath: string,
  comment: Comment,
  actions: CommentActions,
): HTMLElement {
  const item = el("div", { classes: ["outdated-comment"] });
  item.setAttribute("data-comment", comment.id);

  const context = lastGoodContext(comment);
  if (context.length > 0) {
    const hunk = el("div", { classes: ["outdated-hunk"] });
    for (const line of context) {
      hunk.appendChild(
        el("div", { classes: ["diff-line", "outdated-line"], text: line }),
      );
    }
    item.appendChild(hunk);
  }

  item.appendChild(createCommentThread(filePath, [comment], actions));
  return item;
}

// a line-anchored comment thread carrying a `data-line` attribute, so an
// incremental update can find and replace exactly this thread without
// re-rendering the diff.
export function lineCommentThread(
  filePath: string,
  lineNo: number,
  comments: Comment[],
  actions: CommentActions,
): HTMLElement {
  const thread = createCommentThread(filePath, comments, actions);
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
  actions: CommentActions,
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
          createCommentElement(filePath, reply, multipleAuthors, actions),
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
        onClick: () => actions.onReply(filePath, commentID),
      }),
    );
  }
  actionsElem.appendChild(
    el("button", {
      text: "Edit",
      onClick: () => actions.onEdit(filePath, commentID, comment.content),
    }),
  );
  if (!isReply) {
    if (comment.status === "active") {
      actionsElem.appendChild(
        el("button", {
          text: "Resolve",
          onClick: () => actions.onResolve(filePath, commentID),
        }),
      );
      actionsElem.appendChild(
        el("button", {
          text: "Ignore",
          onClick: () => actions.onIgnore(filePath, commentID),
        }),
      );
    } else {
      actionsElem.appendChild(
        el("button", {
          text: "Reactivate",
          onClick: () => actions.onReactivate(filePath, commentID),
        }),
      );
    }
  }
  actionsElem.appendChild(
    el("button", {
      classes: ["delete-btn"],
      text: "Delete",
      onClick: () => actions.onDelete(filePath, commentID),
    }),
  );

  elem.appendChild(actionsElem);
  return elem;
}
