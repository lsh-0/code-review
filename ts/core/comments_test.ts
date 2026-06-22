import { assertEquals } from "@std/assert";
import {
  anchorIsAdrift,
  authorLabel,
  commentRootLine,
  currentLineNumber,
  fileCommentStatus,
  getCommentsForLine,
  getReplies,
  isOutdated,
  lastGoodContext,
  outdatedComments,
  rootComments,
  threadAuthors,
} from "./comments.ts";
import type { Anchor, Comment, CommentStatus } from "./types.ts";

function comment(over: Partial<Comment>): Comment {
  return {
    id: "",
    author: "",
    content: "",
    status: "active",
    ...over,
  };
}

// a located anchor: it carries a context window, so it is not adrift and its
// `line_number` is meaningful.
function goodAnchor(line: number): Anchor {
  return { blob: "x", line_number: line, offset: 0, context: ["ctx"] };
}

// an adrift anchor: no context window, so its `line_number` is not meaningful.
function adriftAnchor(): Anchor {
  return { blob: "y", line_number: 0 };
}

// a root comment located at `line` via a single good anchor.
function root(id: string, status: CommentStatus, line: number): Comment {
  return comment({ id, status, anchors: [goodAnchor(line)] });
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

Deno.test("getCommentsForLine: a comment matches its current anchor's line", () => {
  // most-recent good anchor moved the comment from line 5 to line 8.
  const given = [
    comment({ id: "a", anchors: [goodAnchor(5), goodAnchor(8)] }),
  ];
  assertEquals(getCommentsForLine(given, 8).map((c) => c.id), ["a"]);
  // it no longer matches the line of an older anchor.
  assertEquals(getCommentsForLine(given, 5).map((c) => c.id), []);
});

Deno.test("getCommentsForLine: an outdated comment matches no line", () => {
  // good anchor at line 5, then adrift: the comment is outdated.
  const given = [
    comment({ id: "a", anchors: [goodAnchor(5), adriftAnchor()] }),
  ];
  // not returned for the adrift line_number (0), the last good line (5), or any.
  assertEquals(getCommentsForLine(given, 0), []);
  assertEquals(getCommentsForLine(given, 5), []);
});

Deno.test("anchorIsAdrift: true with empty or absent context", () => {
  assertEquals(anchorIsAdrift({ blob: "x", line_number: 0 }), true);
  assertEquals(
    anchorIsAdrift({ blob: "x", line_number: 0, context: [] }),
    true,
  );
});

Deno.test("anchorIsAdrift: false when a context window is present", () => {
  assertEquals(anchorIsAdrift(goodAnchor(3)), false);
});

Deno.test("currentLineNumber: most-recent good anchor's line", () => {
  const given = comment({ anchors: [goodAnchor(5), goodAnchor(9)] });
  assertEquals(currentLineNumber(given), 9);
});

Deno.test("currentLineNumber: 0 when the current anchor is adrift", () => {
  const given = comment({ anchors: [goodAnchor(5), adriftAnchor()] });
  assertEquals(currentLineNumber(given), 0);
});

Deno.test("currentLineNumber: 0 for a comment with no anchors", () => {
  assertEquals(currentLineNumber(reply("b", "a")), 0);
});

Deno.test("isOutdated: true when the most-recent anchor is adrift", () => {
  const given = comment({ anchors: [goodAnchor(5), adriftAnchor()] });
  assertEquals(isOutdated(given), true);
});

Deno.test("isOutdated: false when the most-recent anchor is good", () => {
  const given = comment({ anchors: [adriftAnchor(), goodAnchor(5)] });
  assertEquals(isOutdated(given), false);
});

Deno.test("isOutdated: false for a comment with no anchors", () => {
  assertEquals(isOutdated(reply("b", "a")), false);
});

Deno.test("isOutdated: true when every anchor is adrift", () => {
  const given = comment({ anchors: [adriftAnchor(), adriftAnchor()] });
  assertEquals(isOutdated(given), true);
});

Deno.test("lastGoodContext: context of the most recent good anchor", () => {
  const given = comment({
    anchors: [
      { blob: "a", line_number: 5, context: ["old"] },
      { blob: "b", line_number: 8, context: ["new"] },
      adriftAnchor(),
    ],
  });
  assertEquals(lastGoodContext(given), ["new"]);
});

Deno.test("lastGoodContext: [] when no anchor is good", () => {
  const given = comment({ anchors: [adriftAnchor(), adriftAnchor()] });
  assertEquals(lastGoodContext(given), []);
});

Deno.test("lastGoodContext: [] for a comment with no anchors", () => {
  assertEquals(lastGoodContext(reply("b", "a")), []);
});

Deno.test("outdatedComments: only outdated roots, never replies", () => {
  const outdated = comment({
    id: "a",
    anchors: [goodAnchor(5), adriftAnchor()],
  });
  const live = root("b", "active", 7);
  // a reply with an adrift anchor is still excluded: it has a parent.
  const adriftReply = comment({
    id: "c",
    parent_id: "a",
    anchors: [adriftAnchor()],
  });
  const given = [outdated, live, adriftReply];
  assertEquals(outdatedComments(given).map((c) => c.id), ["a"]);
});

Deno.test("outdatedComments: empty when no root is outdated", () => {
  const given = [root("a", "active", 5), reply("b", "a")];
  assertEquals(outdatedComments(given), []);
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
