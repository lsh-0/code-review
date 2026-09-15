// DOM integration tests for imported feedback: the action set an imported comment
// offers, and how source visibility affects the rendered surfaces and the file-list
// status pills. Run with --allow-read --allow-net (deno-dom WASM init).

import { assert, assertEquals } from "@std/assert";
import { setupDom } from "./dom_test_setup.ts";
import { renderFileList } from "./filelist.ts";
import { renderDiff } from "./hunks.ts";
import { renderOverview } from "./overview.ts";
import { applyMutation } from "./mutate.ts";
import { type CommentActions, createCommentThread } from "./comments.ts";
import { state } from "./state.ts";
import type { Anchor, Comment, DiffFile } from "../core/types.ts";
import { LineAdded, localSourceID } from "../core/types.ts";

const noopActions: CommentActions = {
  onReply: () => {},
  onEdit: () => {},
  onResolve: () => {},
  onIgnore: () => {},
  onReactivate: () => {},
  onDelete: () => {},
};

const noopFileListCb = {
  onSelectFile: () => {},
  onSelectOverview: () => {},
};

function goodAnchor(line: number): Anchor {
  return { blob: "x", line_number: line, offset: 0, context: ["ctx"] };
}

function local(over: Partial<Comment> = {}): Comment {
  return {
    id: "local-1",
    author: "Test User",
    content: "a local note",
    status: "active",
    ...over,
  };
}

function imported(over: Partial<Comment> = {}): Comment {
  return {
    id: "imported-1",
    author: "review-bot (bot)",
    content: "an imported finding",
    status: "active",
    origin: { source: "bb:1430", foreign_id: "856483458" },
    ...over,
  };
}

// a one-line diff file, for rendering a line thread against.
function oneLineFile(path: string, line: number): DiffFile {
  return {
    Path: path,
    Binary: false,
    Hunks: [{
      OldStart: line,
      OldLines: 1,
      NewStart: line,
      NewLines: 1,
      Lines: [{
        Type: LineAdded,
        Content: "added",
        OldLineNo: 0,
        NewLineNo: line,
      }],
    }],
  };
}

function buttonTexts(elem: Element): string[] {
  return Array.from(elem.querySelectorAll("button")).map((b) =>
    (b.textContent ?? "").trim()
  );
}

Deno.test("comment thread: an imported comment offers no Edit or Delete", () => {
  setupDom();
  const given = imported({ anchors: [goodAnchor(1)] });
  state.commentsCache.set("a.go", [given]);
  state.sources = [{
    id: "bb:1430",
    label: "Bitbucket #1430",
    pull_request: 1430,
    source_branch: "feature",
    target_branch: "master",
    imported_at: "t1",
  }];

  const thread = createCommentThread("a.go", [given], noopActions);
  const actual = buttonTexts(thread);

  assertEquals(actual.includes("Edit"), false);
  assertEquals(actual.includes("Delete"), false);
  assertEquals(actual.includes("Reply"), true);
  assertEquals(actual.includes("Resolve"), true);
  assertEquals(actual.includes("Ignore"), true);
});

Deno.test("comment thread: a local comment keeps Edit and Delete", () => {
  setupDom();
  const given = local({ anchors: [goodAnchor(1)] });
  state.commentsCache.set("a.go", [given]);

  const actual = buttonTexts(createCommentThread("a.go", [given], noopActions));

  assertEquals(actual.includes("Edit"), true);
  assertEquals(actual.includes("Delete"), true);
});

Deno.test("comment thread: an imported comment names its source", () => {
  setupDom();
  const given = imported({ anchors: [goodAnchor(1)] });
  state.commentsCache.set("a.go", [given]);
  state.sources = [{
    id: "bb:1430",
    label: "Bitbucket #1430 — a title",
    pull_request: 1430,
    source_branch: "feature",
    target_branch: "master",
    imported_at: "t1",
  }];

  const thread = createCommentThread("a.go", [given], noopActions);
  const origin = thread.querySelector(".comment-origin");
  if (!origin) {
    throw new Error("expected the imported comment to name its source");
  }
  assertEquals(origin.textContent, "Bitbucket #1430 — a title");
});

Deno.test("comment thread: an imported comment carries the foreign class", () => {
  setupDom();
  const given = imported({ anchors: [goodAnchor(1)] });
  state.commentsCache.set("a.go", [given]);

  const thread = createCommentThread("a.go", [given], noopActions);

  assert(
    thread.querySelector(".comment-foreign"),
    "expected the foreign class",
  );
});

// a local reply under an imported root is a local comment, so the guards must not
// extend to it.
Deno.test("comment thread: a local reply under an imported root keeps Edit and Delete", () => {
  setupDom();
  const root = imported({ anchors: [goodAnchor(1)] });
  const reply = local({ id: "reply-1", parent_id: "imported-1" });
  state.commentsCache.set("a.go", [root, reply]);

  const thread = createCommentThread("a.go", [root], noopActions);
  const replyElem = thread.querySelector(".comment-reply");
  if (!replyElem) {
    throw new Error("expected the reply to render");
  }
  const actual = buttonTexts(replyElem);
  assertEquals(actual.includes("Edit"), true);
  assertEquals(actual.includes("Delete"), true);
});

Deno.test("diff: hiding a source removes its thread from the rendered lines", () => {
  const doc = setupDom();
  state.diffFiles = [oneLineFile("a.go", 4)];
  state.commentsCache.set("a.go", [imported({ anchors: [goodAnchor(4)] })]);

  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });
  assert(
    doc.querySelector(".comment-thread"),
    "expected the imported thread to render while visible",
  );

  state.hiddenSources = new Set(["bb:1430"]);
  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });

  assertEquals(
    doc.querySelectorAll(".comment-thread").length,
    0,
    "expected no thread once its source is hidden",
  );
});

Deno.test("diff: hiding local feedback leaves a visible imported thread in place", () => {
  const doc = setupDom();
  state.diffFiles = [oneLineFile("a.go", 4)];
  state.commentsCache.set("a.go", [imported({ anchors: [goodAnchor(4)] })]);
  state.hiddenSources = new Set([localSourceID]);

  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });

  assert(
    doc.querySelector(".comment-thread"),
    "expected the imported thread to survive hiding local feedback",
  );
});

Deno.test("file list: a file whose only comments are hidden shows no pill", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "a.go", Hunks: [], Binary: false }];
  state.commentsCache.set("a.go", [imported({ anchors: [goodAnchor(1)] })]);

  renderFileList(noopFileListCb);
  let item = doc.querySelector("#files .file-item");
  assert(item, "expected the file item to render");
  assert(
    item.classList.contains("has-comments-active"),
    "expected an active pill while the source is visible",
  );

  state.hiddenSources = new Set(["bb:1430"]);
  renderFileList(noopFileListCb);

  item = doc.querySelector("#files .file-item");
  assert(item, "expected the file item to still render");
  assertEquals(
    item.classList.contains("has-comments-active"),
    false,
    "expected no pill once the file's only comments are hidden",
  );
});

// the pill must describe the visible subset, not the whole comment list.
Deno.test("file list: a mixed file's pill reflects only the visible source", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "a.go", Hunks: [], Binary: false }];
  state.commentsCache.set("a.go", [
    imported({ id: "i-1", status: "active", anchors: [goodAnchor(1)] }),
    local({ id: "l-1", status: "resolved", anchors: [goodAnchor(2)] }),
  ]);
  state.hiddenSources = new Set(["bb:1430"]);

  renderFileList(noopFileListCb);

  const item = doc.querySelector("#files .file-item");
  assert(item, "expected the file item to render");
  assert(
    item.classList.contains("has-comments-resolved"),
    "expected the pill to describe the visible resolved comment",
  );
  assertEquals(item.classList.contains("has-comments-active"), false);
});

// the overview receives its comments as arguments from the bridge rather than
// through `commentsFor`, so it is the one surface that must filter for itself.
Deno.test("overview: hiding a source removes its review-level thread", async () => {
  const doc = setupDom();
  const reviewComment = imported({ id: "top-1" });
  state.commentsCache.set("", [reviewComment]);
  state.hiddenSources = new Set(["bb:1430"]);

  await renderOverview([reviewComment], [], {
    diff: { actions: noopActions, onAddComment: () => {} },
    actions: noopActions,
    onSelectFile: () => {},
    onAddReviewComment: () => {},
  });

  assertEquals(
    doc.querySelectorAll(".overview-review .comment").length,
    0,
    "expected no review-level comment once its source is hidden",
  );
});

Deno.test("overview: a visible review-level thread still renders", async () => {
  const doc = setupDom();
  const reviewComment = imported({ id: "top-1" });
  state.commentsCache.set("", [reviewComment]);

  await renderOverview([reviewComment], [], {
    diff: { actions: noopActions, onAddComment: () => {} },
    actions: noopActions,
    onSelectFile: () => {},
    onAddReviewComment: () => {},
  });

  assert(
    doc.querySelector(".overview-review .comment"),
    "expected the review-level comment to render while visible",
  );
});

// a file whose feedback all belongs to a hidden source has nothing to show, so it
// drops out of the overview rather than appearing with an empty body.
Deno.test("overview: a file whose comments are all hidden is not listed", async () => {
  const doc = setupDom();
  const fileComment = imported({ anchors: [goodAnchor(4)] });
  state.diffFiles = [oneLineFile("a.go", 4)];
  state.commentsCache.set("a.go", [fileComment]);
  state.hiddenSources = new Set(["bb:1430"]);

  await renderOverview([], [{ path: "a.go", comments: [fileComment] }], {
    diff: { actions: noopActions, onAddComment: () => {} },
    actions: noopActions,
    onSelectFile: () => {},
    onAddReviewComment: () => {},
  });

  // the review-level section is always present; no per-file section should be.
  assertEquals(
    doc.querySelectorAll(".overview-file:not(.overview-review)").length,
    0,
    "expected the file section to be omitted once its only source is hidden",
  );
});

Deno.test("overview: a file with a visible comment is listed", async () => {
  const doc = setupDom();
  const fileComment = imported({ anchors: [goodAnchor(4)] });
  state.diffFiles = [oneLineFile("a.go", 4)];
  state.commentsCache.set("a.go", [fileComment]);

  await renderOverview([], [{ path: "a.go", comments: [fileComment] }], {
    diff: { actions: noopActions, onAddComment: () => {} },
    actions: noopActions,
    onSelectFile: () => {},
    onAddReviewComment: () => {},
  });

  assertEquals(
    doc.querySelectorAll(".overview-file:not(.overview-review)").length,
    1,
    "expected the file section to render while its source is visible",
  );
});

// the untethered block reads its comments through `commentsFor`, so it filters with
// every other surface. Asserted rather than assumed: the overview leak came from an
// unverified path of exactly this shape.
Deno.test("diff: hiding a source removes its untethered comments", () => {
  const doc = setupDom();
  state.diffFiles = [oneLineFile("a.go", 4)];
  // an adrift anchor with a last-good window: the comment renders untethered.
  state.commentsCache.set("a.go", [
    imported({
      anchors: [
        { blob: "x", line_number: 4, offset: 0, context: ["gone"] },
        { blob: "y", line_number: 0 },
      ],
    }),
  ]);

  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });
  assert(
    doc.querySelector(".outdated-comments"),
    "expected the untethered block while the source is visible",
  );

  state.hiddenSources = new Set(["bb:1430"]);
  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });

  assertEquals(
    doc.querySelectorAll(".outdated-comments").length,
    0,
    "expected no untethered block once its only source is hidden",
  );
});

// `applyMutation` deliberately recomputes the pill from the visible cache instead of
// using the backend's `file_status`, which is computed over every comment and knows
// nothing about what the reviewer has hidden. Without this the pill would come back
// for hidden feedback on the next mutation.
Deno.test("mutation: the pill reflects visible sources, not the backend's status", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "a.go", Hunks: [], Binary: false }];
  state.commentsCache.set("a.go", [imported({ anchors: [goodAnchor(1)] })]);
  state.hiddenSources = new Set(["bb:1430"]);
  renderFileList(noopFileListCb);

  // the backend reports "active" over every comment, hidden ones included.
  applyMutation({
    file_path: "a.go",
    line_number: 1,
    comments: [imported({ anchors: [goodAnchor(1)] })],
    file_status: "active",
  }, { actions: noopActions, rebuildOverview: () => {} });

  const item = doc.querySelector("#files .file-item");
  if (!item) {
    throw new Error("expected the file item to render");
  }
  assertEquals(
    item.classList.contains("has-comments-active"),
    false,
    "expected the pill to ignore the backend status for a hidden source",
  );
});

Deno.test("mutation: the pill is applied when the source is visible", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "a.go", Hunks: [], Binary: false }];
  state.commentsCache.set("a.go", [imported({ anchors: [goodAnchor(1)] })]);
  renderFileList(noopFileListCb);

  applyMutation({
    file_path: "a.go",
    line_number: 1,
    comments: [imported({ anchors: [goodAnchor(1)] })],
    file_status: "active",
  }, { actions: noopActions, rebuildOverview: () => {} });

  const item = doc.querySelector("#files .file-item");
  if (!item) {
    throw new Error("expected the file item to render");
  }
  assert(
    item.classList.contains("has-comments-active"),
    "expected the active pill for a visible source",
  );
});

// a file that stays listed because another source is visible must still lose the
// hidden source's threads from its rendered hunks.
Deno.test("diff: a mixed file loses only the hidden source's threads", () => {
  const doc = setupDom();
  state.diffFiles = [oneLineFile("a.go", 4)];
  state.commentsCache.set("a.go", [
    imported({ id: "i-1", anchors: [goodAnchor(4)] }),
    local({ id: "l-1", anchors: [goodAnchor(4)] }),
  ]);
  state.hiddenSources = new Set(["bb:1430"]);

  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });

  const comments = doc.querySelectorAll(".comment-thread .comment");
  assertEquals(comments.length, 1, "expected only the local comment to render");
  assertEquals(
    doc.querySelectorAll(".comment-foreign").length,
    0,
    "expected no imported comment to render",
  );
});

// the rendered reply half of the reply-follows-root rule, which is otherwise only
// covered by unit tests over the predicate.
Deno.test("diff: a local reply renders under a visible imported root when local feedback is hidden", () => {
  const doc = setupDom();
  state.diffFiles = [oneLineFile("a.go", 4)];
  state.commentsCache.set("a.go", [
    imported({ id: "i-1", anchors: [goodAnchor(4)] }),
    local({ id: "r-1", parent_id: "i-1" }),
  ]);
  state.hiddenSources = new Set([localSourceID]);

  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });

  assert(
    doc.querySelector(".comment-reply"),
    "expected the local reply to render inside the visible imported thread",
  );
});
