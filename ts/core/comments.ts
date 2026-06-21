// pure comment-thread logic: file comment status, root-comment lines, reply
// grouping, per-line lookup, and thread authorship. Every function takes the
// flat comment list as input rather than reading a global cache, so they are
// pure and testable without a DOM. The render layer holds the cache and passes
// the relevant list in.

import type { Comment, FileStatus } from "./types.ts";

// the root comments (no parent) from a flat comment list.
export function rootComments(comments: Comment[]): Comment[] {
  return comments.filter((c) => !c.parent_id);
}

// the replies whose `parent_id` is `parentID`, in stored order.
export function getReplies(comments: Comment[], parentID: string): Comment[] {
  return comments.filter((c) => c.parent_id === parentID);
}

// the root comments anchored to a given new-file line. Replies (with a
// `parent_id`) are rendered under their root, not against a line, so a line
// thread is built only from roots.
export function getCommentsForLine(
  comments: Comment[],
  lineNumber: number,
): Comment[] {
  return comments.filter((c) => !c.parent_id && c.line_number === lineNumber);
}

// the aggregate review status of a file from its flat comment list, matching
// the Go `model.FileCommentStatus`: `active` if any root is active, else
// `resolved` if every root is resolved, else `ignored` if any root is ignored,
// else `none`. Replies carry no status and are ignored.
export function fileCommentStatus(comments: Comment[]): FileStatus {
  let hasActive = false;
  let hasIgnored = false;
  let allResolved = true;
  let rootCount = 0;

  for (const comment of comments) {
    if (comment.parent_id) {
      continue;
    }
    rootCount++;
    switch (comment.status) {
      case "active":
        hasActive = true;
        allResolved = false;
        break;
      case "ignored":
        hasIgnored = true;
        allResolved = false;
        break;
    }
  }

  if (hasActive) {
    return "active";
  }
  if (allResolved && rootCount > 0) {
    return "resolved";
  }
  if (hasIgnored) {
    return "ignored";
  }
  return "none";
}

// the line number of a comment's root within a flat comment list, matching the
// Go `model.CommentRootLine`. For a root comment that is its own line; for a
// reply it is the root's line. Returns 0 if the comment is absent or its root is
// missing (review-level comments carry no line and report 0).
export function commentRootLine(
  comments: Comment[],
  commentID: string,
): number {
  const comment = comments.find((c) => c.id === commentID);
  if (!comment) {
    return 0;
  }
  if (!comment.parent_id) {
    return comment.line_number;
  }
  const root = comments.find((c) => c.id === comment.parent_id);
  return root ? root.line_number : 0;
}

// the distinct authors across a root comment and its replies. Used to decide
// whether to label authors: a single-author thread needs no labels, but once
// two or more people have written in it, each entry is labelled.
export function threadAuthors(comments: Comment[], root: Comment): Set<string> {
  const authors = new Set<string>();
  if (root.author) {
    authors.add(root.author);
  }
  for (const reply of getReplies(comments, root.id)) {
    if (reply.author) {
      authors.add(reply.author);
    }
  }
  return authors;
}

// the author label for a comment when its thread has multiple authors: the
// reviewer's own comments read as "(user)", everyone else by name. Empty when
// the thread has a single author or the comment has no author.
export function authorLabel(
  comment: Comment,
  multipleAuthors: boolean,
  currentUser: string,
): string {
  if (!multipleAuthors || !comment.author) {
    return "";
  }
  if (comment.author === currentUser) {
    return "(user)";
  }
  return `(${comment.author})`;
}
