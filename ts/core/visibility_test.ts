import { assertEquals } from "@std/assert";
import {
  commentSource,
  isForeign,
  sourceToggles,
  visibleComments,
} from "./comments.ts";
import type { Comment, Source } from "./types.ts";
import { localSourceID } from "./types.ts";

function local(id: string, parentID?: string): Comment {
  return {
    id,
    parent_id: parentID,
    author: "Test User",
    content: "local",
    status: "active",
    line: 0,
    outdated: false,
  };
}

function imported(id: string, source: string, parentID?: string): Comment {
  return {
    id,
    parent_id: parentID,
    author: "review-bot (bot)",
    content: "imported",
    status: "active",
    line: 0,
    outdated: false,
    origin: source,
  };
}

function idsOf(comments: Comment[]): string[] {
  return comments.map((c) => c.id);
}

Deno.test("commentSource: a local comment belongs to the local source", () => {
  assertEquals(commentSource(local("a")), localSourceID);
});

Deno.test("commentSource: an imported comment names its source", () => {
  assertEquals(commentSource(imported("a", "bb:1430")), "bb:1430");
});

Deno.test("isForeign: distinguishes imported from local", () => {
  assertEquals(isForeign(local("a")), false);
  assertEquals(isForeign(imported("a", "bb:1430")), true);
});

Deno.test("visibleComments: an empty hidden set shows everything", () => {
  const given = [local("a"), imported("b", "bb:1430")];
  assertEquals(idsOf(visibleComments(given, new Set())), ["a", "b"]);
});

Deno.test("visibleComments: hiding local feedback hides local roots", () => {
  const given = [local("a"), imported("b", "bb:1430")];
  const actual = visibleComments(given, new Set([localSourceID]));
  assertEquals(idsOf(actual), ["b"]);
});

Deno.test("visibleComments: hiding a source hides its roots", () => {
  const given = [local("a"), imported("b", "bb:1430")];
  const actual = visibleComments(given, new Set(["bb:1430"]));
  assertEquals(idsOf(actual), ["a"]);
});

Deno.test("visibleComments: hiding one source leaves another visible", () => {
  const given = [imported("a", "bb:1430"), imported("b", "gh:88")];
  const actual = visibleComments(given, new Set(["bb:1430"]));
  assertEquals(idsOf(actual), ["b"]);
});

// a reply rendered without the comment it answers has no context, so a local reply
// is hidden with the imported thread it belongs to.
Deno.test("visibleComments: a local reply is hidden with its imported root", () => {
  const given = [imported("a", "bb:1430"), local("b", "a")];
  const actual = visibleComments(given, new Set(["bb:1430"]));
  assertEquals(idsOf(actual), []);
});

// hiding local feedback must not strip a local reply out of a visible imported
// thread: that reply is part of the thread being shown.
Deno.test("visibleComments: a local reply to a visible imported root survives hiding local feedback", () => {
  const given = [imported("a", "bb:1430"), local("b", "a")];
  const actual = visibleComments(given, new Set([localSourceID]));
  assertEquals(idsOf(actual), ["a", "b"]);
});

Deno.test("visibleComments: hiding local feedback hides a local root and its reply", () => {
  const given = [local("a"), local("b", "a")];
  const actual = visibleComments(given, new Set([localSourceID]));
  assertEquals(idsOf(actual), []);
});

Deno.test("visibleComments: an imported reply is hidden with its source", () => {
  const given = [imported("a", "bb:1430"), imported("b", "bb:1430", "a")];
  const actual = visibleComments(given, new Set(["bb:1430"]));
  assertEquals(idsOf(actual), []);
});

// a reply whose root is not in the list cannot resolve its thread's source, so it
// falls back to its own rather than disappearing unexplained.
Deno.test("visibleComments: a reply with a missing root falls back to its own source", () => {
  const given = [local("b", "gone")];
  assertEquals(idsOf(visibleComments(given, new Set(["bb:1430"]))), ["b"]);
  assertEquals(idsOf(visibleComments(given, new Set([localSourceID]))), []);
});

Deno.test("visibleComments: hiding every source shows nothing", () => {
  const given = [local("a"), imported("b", "bb:1430")];
  const actual = visibleComments(given, new Set([localSourceID, "bb:1430"]));
  assertEquals(idsOf(actual), []);
});

// {1 sourceToggles: the entries the visibility control renders}

const bb: Source = {
  id: "bb:1430",
  label: "Bitbucket #1430 — a title",
  pull_request: 1430,
  source_branch: "feature",
  target_branch: "master",
  imported_at: "t1",
};

const gh: Source = {
  id: "gh:88",
  label: "GitHub #88",
  pull_request: 88,
  source_branch: "feature",
  target_branch: "main",
  imported_at: "t2",
};

Deno.test("sourceToggles: local feedback is the first entry", () => {
  const actual = sourceToggles([bb], new Set());

  assertEquals(actual.length, 2);
  assertEquals(actual[0].id, localSourceID);
  assertEquals(actual[0].label, "Local feedback");
  assertEquals(actual[1].id, "bb:1430");
  assertEquals(actual[1].label, "Bitbucket #1430 — a title");
});

Deno.test("sourceToggles: every entry starts visible", () => {
  const actual = sourceToggles([bb, gh], new Set());

  assertEquals(actual.map((e) => e.visible), [true, true, true]);
});

Deno.test("sourceToggles: a hidden source reports not visible", () => {
  const actual = sourceToggles([bb, gh], new Set(["bb:1430"]));

  assertEquals(actual.map((e) => [e.id, e.visible]), [
    [localSourceID, true],
    ["bb:1430", false],
    ["gh:88", true],
  ]);
});

Deno.test("sourceToggles: hiding local feedback reports it not visible", () => {
  const actual = sourceToggles([bb], new Set([localSourceID]));

  assertEquals(actual[0].visible, false);
  assertEquals(actual[1].visible, true);
});

// with nothing imported the control would hold one toggle whose only effect is to
// hide every comment in the review, so there is no choice to offer.
Deno.test("sourceToggles: no imports yields no entries", () => {
  assertEquals(sourceToggles([], new Set()), []);
});

Deno.test("sourceToggles: imported sources keep their given order after local", () => {
  const actual = sourceToggles([gh, bb], new Set());

  assertEquals(actual.map((e) => e.id), [localSourceID, "gh:88", "bb:1430"]);
});
