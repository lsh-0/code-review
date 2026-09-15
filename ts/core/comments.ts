// pure comment-thread logic: file comment status, reply grouping, per-line
// lookup, and thread authorship. Every function takes the flat comment list as
// input rather than reading a global cache, so they are pure and testable
// without a DOM. The render layer holds the cache and passes the relevant list
// in. Placement and outdatedness arrive backend-derived on each comment
// (`line`, `outdated`, `context`); nothing here re-derives them.

import type { Comment, FileStatus, Source } from "./types.ts";
import { localSourceID } from "./types.ts";

// the source a comment belongs to: the source it was imported from, or
// `localSourceID` when it was written here. Every comment belongs to exactly one,
// which is what lets the visibility toggles treat local feedback as one source
// among the imports.
export function commentSource(comment: Comment): string {
  return comment.origin || localSourceID;
}

// whether a comment was imported from an external review, in which case its
// content is read-only. Mirrors the Go `Comment.IsForeign`.
export function isForeign(comment: Comment): boolean {
  return !!comment.origin;
}

// one entry in the source-visibility control: the id it toggles and the label it
// shows.
export interface SourceToggle {
  id: string;
  label: string;
  visible: boolean;
}

// the source-visibility entries for a review, local feedback first.
//
// Local feedback is an entry here rather than a special case, so it toggles by the
// same mechanism as any import. Returns an empty list when nothing has been
// imported: the control would then hold a single toggle whose only effect is to hide
// every comment in the review, so there is no choice to offer.
export function sourceToggles(
  sources: Source[],
  hiddenSources: Set<string>,
): SourceToggle[] {
  if (sources.length === 0) {
    return [];
  }
  return [
    { id: localSourceID, label: "Local feedback" },
    ...sources.map((s) => ({ id: s.id, label: s.label })),
  ].map((entry) => ({ ...entry, visible: !hiddenSources.has(entry.id) }));
}

// the comments of a surface that belong to a visible source.
//
// A root is visible when its own source is visible. A reply's visibility follows
// its root's source, not its own: a local reply to an imported comment is part of
// that imported thread, so hiding the import hides the reply with it, and hiding
// local feedback leaves it in place. A reply whose root is missing from `comments`
// falls back to its own source.
export function visibleComments(
  comments: Comment[],
  hiddenSources: Set<string>,
): Comment[] {
  if (hiddenSources.size === 0) {
    return comments;
  }
  return comments.filter((c) => {
    if (!c.parent_id) {
      return !hiddenSources.has(commentSource(c));
    }
    const root = comments.find((candidate) => candidate.id === c.parent_id);
    return !hiddenSources.has(commentSource(root ?? c));
  });
}

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
  return comments.filter(
    (c) => !c.parent_id && !c.outdated && c.line === lineNumber,
  );
}

// the root comments that can no longer be placed against the current file.
// These render untethered at the top of the file view rather than against a
// line. Replies stay with their root and are excluded here.
export function outdatedComments(comments: Comment[]): Comment[] {
  return comments.filter((c) => !c.parent_id && c.outdated);
}

// the root comments attached to the file but to no line: line 0 and never
// outdated. These are the "top-level" file comments (the anchorless shape
// review-level comments also use), rendered at the top of the file view above
// the untethered block — the only way to comment on a file with no commentable
// line, such as a binary file. This is the third bucket beside
// `getCommentsForLine` (line-anchored) and `outdatedComments` (adrift); the
// three are disjoint. Replies are excluded.
export function topLevelComments(comments: Comment[]): Comment[] {
  return comments.filter(
    (c) => !c.parent_id && !c.outdated && c.line === 0,
  );
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
