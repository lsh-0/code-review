import { assertEquals } from "@std/assert";
import {
  authorLabel,
  commentRootLine,
  fileCommentStatus,
  getCommentsForLine,
  getReplies,
  rootComments,
  threadAuthors,
} from "./comments.ts";
import type { Comment, CommentStatus } from "./types.ts";

function comment(over: Partial<Comment>): Comment {
  return {
    id: "",
    author: "",
    content: "",
    line_number: 0,
    status: "active",
    context_before: "",
    context_line: "",
    context_after: "",
    ...over,
  };
}

function root(id: string, status: CommentStatus, line: number): Comment {
  return comment({ id, status, line_number: line });
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

Deno.test("commentRootLine: a root reports its own line", () => {
  const given = [root("a", "active", 12)];
  assertEquals(commentRootLine(given, "a"), 12);
});

Deno.test("commentRootLine: a reply reports its root's line", () => {
  const given = [root("a", "active", 12), reply("b", "a")];
  assertEquals(commentRootLine(given, "b"), 12);
});

Deno.test("commentRootLine: an absent comment reports 0", () => {
  assertEquals(commentRootLine([], "missing"), 0);
});

Deno.test("commentRootLine: a reply with a missing root reports 0", () => {
  const given = [reply("b", "gone")];
  assertEquals(commentRootLine(given, "b"), 0);
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
