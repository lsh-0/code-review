import { assertEquals } from "@std/assert";
import {
  authorLabel,
  fileCommentStatus,
  getCommentsForLine,
  getReplies,
  outdatedComments,
  rootComments,
  threadAuthors,
  topLevelComments,
} from "./comments.ts";
import type { Comment, CommentStatus } from "./types.ts";

function comment(over: Partial<Comment>): Comment {
  return {
    id: "",
    author: "",
    content: "",
    status: "active",
    line: 0,
    outdated: false,
    ...over,
  };
}

// a root comment placed at `line` (backend-derived, not outdated).
function root(id: string, status: CommentStatus, line: number): Comment {
  return comment({ id, status, line });
}

// an outdated root: no placement, carrying the last good context the
// untethered block renders.
function outdated(id: string, context: string[] = ["ctx"]): Comment {
  return comment({ id, outdated: true, context });
}

function reply(id: string, parent: string, author = ""): Comment {
  return comment({ id, parent_id: parent, author });
}

Deno.test("rootComments: drops replies", () => {
  const given = [root("a", "active", 1), reply("b", "a")];
  const actual = rootComments(given).map((c) => c.id);
  assertEquals(actual, ["a"]);
});

Deno.test("getReplies: returns replies of a parent in order", () => {
  const given = [
    root("a", "active", 1),
    reply("b", "a"),
    reply("c", "a"),
    reply("d", "x"),
  ];
  const actual = getReplies(given, "a").map((c) => c.id);
  assertEquals(actual, ["b", "c"]);
});

Deno.test("getCommentsForLine: roots on the line only, never replies", () => {
  const given = [
    root("a", "active", 5),
    root("b", "active", 7),
    reply("c", "a"),
  ];
  const actual = getCommentsForLine(given, 5).map((c) => c.id);
  assertEquals(actual, ["a"]);
});

Deno.test("getCommentsForLine: an outdated comment matches no line", () => {
  // outdated comments report line 0 but are excluded even there.
  const given = [outdated("a")];
  assertEquals(getCommentsForLine(given, 0), []);
  assertEquals(getCommentsForLine(given, 5), []);
});

Deno.test("outdatedComments: only outdated roots, never replies", () => {
  const live = root("b", "active", 7);
  // a reply is excluded even if flagged outdated: it has a parent.
  const outdatedReply = comment({ id: "c", parent_id: "a", outdated: true });
  const given = [outdated("a"), live, outdatedReply];
  assertEquals(outdatedComments(given).map((c) => c.id), ["a"]);
});

Deno.test("outdatedComments: empty when no root is outdated", () => {
  const given = [root("a", "active", 5), reply("b", "a")];
  assertEquals(outdatedComments(given), []);
});

Deno.test("topLevelComments: only line-0 live roots, never lines/outdated/replies", () => {
  const topLevel = comment({ id: "a", content: "about the whole file" });
  const line = root("b", "active", 7);
  const replyToTopLevel = reply("d", "a");
  const given = [topLevel, line, outdated("c"), replyToTopLevel];
  assertEquals(topLevelComments(given).map((c) => c.id), ["a"]);
});

Deno.test("topLevelComments: the three buckets are disjoint", () => {
  const topLevel = comment({ id: "a" });
  const line = root("b", "active", 7);
  const given = [topLevel, line, outdated("c")];

  assertEquals(topLevelComments(given).map((c) => c.id), ["a"]);
  assertEquals(getCommentsForLine(given, 7).map((c) => c.id), ["b"]);
  assertEquals(outdatedComments(given).map((c) => c.id), ["c"]);
});

Deno.test("topLevelComments: empty when every root is line-anchored", () => {
  const given = [root("a", "active", 5), reply("b", "a")];
  assertEquals(topLevelComments(given), []);
});

Deno.test("fileCommentStatus: any active root is active", () => {
  const given = [root("a", "resolved", 1), root("b", "active", 2)];
  assertEquals(fileCommentStatus(given), "active");
});

Deno.test("fileCommentStatus: all roots resolved is resolved", () => {
  const given = [root("a", "resolved", 1), root("b", "resolved", 2)];
  assertEquals(fileCommentStatus(given), "resolved");
});

Deno.test("fileCommentStatus: an ignored root with no active is ignored", () => {
  const given = [root("a", "resolved", 1), root("b", "ignored", 2)];
  assertEquals(fileCommentStatus(given), "ignored");
});

Deno.test("fileCommentStatus: no roots is none", () => {
  const given = [reply("b", "a")];
  assertEquals(fileCommentStatus(given), "none");
});

Deno.test("fileCommentStatus: empty list is none", () => {
  assertEquals(fileCommentStatus([]), "none");
});

Deno.test("threadAuthors: distinct authors across root and replies", () => {
  const given = [
    comment({ id: "a", author: "alice" }),
    reply("b", "a", "bob"),
    reply("c", "a", "alice"),
  ];
  const actual = [...threadAuthors(given, given[0])].sort();
  assertEquals(actual, ["alice", "bob"]);
});

Deno.test("authorLabel: empty when single author", () => {
  const c = comment({ author: "bob" });
  assertEquals(authorLabel(c, false, "me"), "");
});

Deno.test("authorLabel: own comment reads as (user)", () => {
  const c = comment({ author: "me" });
  assertEquals(authorLabel(c, true, "me"), "(user)");
});

Deno.test("authorLabel: other author by name", () => {
  const c = comment({ author: "bob" });
  assertEquals(authorLabel(c, true, "me"), "(bob)");
});
