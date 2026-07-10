// DOM integration tests: render known state into a real (deno-dom) tree and
// assert the produced structure — nodes, classes, attributes, ordering — rather
// than markup strings. This exercises the render path that pure-core tests
// cannot. Run with --allow-read --allow-net (deno-dom WASM init). It asserts our
// rendering produces the right tree, NOT that WebKit paints it (renderer quirks
// stay a manual concern).

import { assert, assertEquals } from "@std/assert";
import { setupDom } from "./dom_test_setup.ts";
import { renderFileList } from "./filelist.ts";
import { renderCombinedDiff, renderDiff } from "./hunks.ts";
import { applyMutation } from "./mutate.ts";
import { type CommentActions, outdatedCommentsBlock } from "./comments.ts";
import { lastGoodContext } from "../core/comments.ts";
import { state } from "./state.ts";
import type { Anchor, Comment, DiffFile } from "../core/types.ts";
import { LineAdded, LineContext } from "../core/types.ts";

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

function comment(over: Partial<Comment>): Comment {
  return {
    id: "c1",
    author: "alice",
    content: "a note",
    status: "active",
    ...over,
  };
}

// a located anchor at `line` carrying a context window, so it is not adrift.
function goodAnchor(line: number, context = ["ctx"]): Anchor {
  return { blob: "x", line_number: line, offset: 0, context };
}

// an adrift anchor: no context, so its `line_number` is not meaningful.
function adriftAnchor(): Anchor {
  return { blob: "y", line_number: 0 };
}

Deno.test("file list: renders one item per diff file in order, with paths", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "a.go", Hunks: [], Binary: false },
    { Path: "b.go", Hunks: [], Binary: false },
  ];

  renderFileList(noopFileListCb);

  const items = doc.querySelectorAll("#files .file-item");
  assertEquals(items.length, 2);
  const paths = Array.from(items).map((i) =>
    (i as unknown as HTMLElement).dataset.path
  );
  assertEquals(paths, ["a.go", "b.go"]);
});

Deno.test("file list: active status adds the has-comments-active pill class", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "a.go", Hunks: [], Binary: false }];
  state.commentsCache.set("a.go", [
    comment({ status: "active", anchors: [goodAnchor(1)] }),
  ]);

  renderFileList(noopFileListCb);

  const item = doc.querySelector("#files .file-item");
  assert(item);
  assert(
    (item as unknown as HTMLElement).classList.contains("has-comments-active"),
  );
});

Deno.test("file list: every file in a multi-file selection is marked active", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "a.go", Hunks: [], Binary: false },
    { Path: "b.go", Hunks: [], Binary: false },
    { Path: "c.go", Hunks: [], Binary: false },
  ];
  state.selectedFiles = ["a.go", "c.go"];

  renderFileList(noopFileListCb);

  const active = Array.from(doc.querySelectorAll("#files .file-item.active"))
    .map((i) => (i as unknown as HTMLElement).dataset.path);
  assertEquals(active, ["a.go", "c.go"]);
});

Deno.test("file list: a marked file renders a checked checkbox", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "a.go", Hunks: [], Binary: false }];
  state.markedFiles = new Set(["a.go"]);

  renderFileList(noopFileListCb);

  const checkbox = doc.querySelector(
    "#files .file-marked",
  ) as unknown as HTMLInputElement;
  assert(checkbox);
  assertEquals(checkbox.checked, true);
});

Deno.test("file list: an active filter renders only matching files", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "ts/render/modals.ts", Hunks: [], Binary: false },
    { Path: "backend/main.go", Hunks: [], Binary: false },
  ];
  state.fileFilter = "modal";

  renderFileList(noopFileListCb);

  const paths = Array.from(doc.querySelectorAll("#files .file-item")).map((i) =>
    (i as unknown as HTMLElement).dataset.path
  );
  assertEquals(paths, ["ts/render/modals.ts"]);
});

Deno.test("file list: the overview entry stays visible under a filter that matches no file", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "backend/main.go", Hunks: [], Binary: false }];
  state.fileFilter = "no-such-file";

  renderFileList(noopFileListCb);

  assertEquals(doc.querySelectorAll("#files .file-item").length, 0);
  assert(doc.querySelector("#overview-footer .overview-item"));
});

Deno.test("file list: grouping by mark renders headings, unmarked group first", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "a.go", Hunks: [], Binary: false },
    { Path: "b.go", Hunks: [], Binary: false },
  ];
  state.markedFiles = new Set(["a.go"]);
  state.fileGrouping = "marked";

  renderFileList(noopFileListCb);

  const headings = Array.from(
    doc.querySelectorAll("#files .file-group-heading"),
  )
    .map((h) => h.textContent);
  assertEquals(headings, ["Unmarked", "Marked"]);

  // the first item under the first heading is the unmarked file.
  const firstItem = doc.querySelector("#files .file-item");
  assertEquals((firstItem as unknown as HTMLElement).dataset.path, "b.go");
});

Deno.test("file list: a flat list (grouping none) renders no group heading", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "a.go", Hunks: [], Binary: false }];

  renderFileList(noopFileListCb);

  assertEquals(doc.querySelectorAll("#files .file-group-heading").length, 0);
});

Deno.test("file list: the overview footer entry is always rendered", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "a.go", Hunks: [], Binary: false }];
  renderFileList(noopFileListCb);
  const entry = doc.querySelector("#overview-footer .overview-item");
  assert(entry);
  assertEquals(entry.textContent, "Review overview");
});

const sampleFile: DiffFile = {
  Path: "a.go",
  Binary: false,
  Hunks: [
    {
      OldStart: 1,
      OldLines: 2,
      NewStart: 1,
      NewLines: 3,
      Lines: [
        {
          Type: LineContext,
          Content: "package main",
          OldLineNo: 1,
          NewLineNo: 1,
        },
        { Type: LineAdded, Content: "// added", OldLineNo: 0, NewLineNo: 2 },
        {
          Type: LineContext,
          Content: "func main() {}",
          OldLineNo: 2,
          NewLineNo: 3,
        },
      ],
    },
  ],
};

Deno.test("diff: renders a hunk header and one row per line", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile)];
  state.commentsCache.set("a.go", []);
  state.currentFile = "a.go";

  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });

  const header = doc.querySelector("#diff-content .hunk-header");
  assert(header);
  assertEquals(header.textContent, "@@ -1,2 +1,3 @@");

  const lines = doc.querySelectorAll("#diff-content .diff-line");
  assertEquals(lines.length, 3);
});

Deno.test("diff: an added line carries the added class and a data-line", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile)];
  state.commentsCache.set("a.go", []);

  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });

  const added = doc.querySelector(
    "#diff-content .diff-line.added",
  ) as unknown as HTMLElement;
  assert(added);
  assertEquals(added.getAttribute("data-line"), "2");
});

Deno.test("diff: a commented line embeds a thread anchored by data-line", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile)];
  state.commentsCache.set("a.go", [
    comment({ id: "x", anchors: [goodAnchor(3)], content: "look here" }),
  ]);

  renderDiff("a.go", { actions: noopActions, onAddComment: () => {} });

  const thread = doc.querySelector(
    '#diff-content .comment-thread[data-line="3"]',
  );
  assert(thread, "expected a thread anchored at line 3");
  assert(thread.textContent?.includes("look here"));

  // the anchored row is marked commented.
  const row = doc.querySelector(
    '#diff-content .diff-line[data-line="3"]',
  ) as unknown as HTMLElement;
  assert(row.classList.contains("commented"));
});

// a second file whose single hunk shares line number 3 with `sampleFile`, so a
// combined view of both stacks two sections that each carry a `data-line="3"`.
const sampleFileB: DiffFile = {
  Path: "b.go",
  Binary: false,
  Hunks: [
    {
      OldStart: 1,
      OldLines: 2,
      NewStart: 1,
      NewLines: 3,
      Lines: [
        {
          Type: LineContext,
          Content: "package other",
          OldLineNo: 1,
          NewLineNo: 1,
        },
        { Type: LineAdded, Content: "// b added", OldLineNo: 0, NewLineNo: 2 },
        {
          Type: LineContext,
          Content: "func other() {}",
          OldLineNo: 2,
          NewLineNo: 3,
        },
      ],
    },
  ],
};

Deno.test("combined diff: a single selected file renders sectionless, like the single-file view", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile)];
  state.commentsCache.set("a.go", []);
  state.selectedFiles = ["a.go"];

  renderCombinedDiff(["a.go"], {
    actions: noopActions,
    onAddComment: () => {},
  });

  // no section wrapper for one file: the hunks sit directly under #diff-content.
  assertEquals(doc.querySelectorAll(".diff-file-section").length, 0);
  assertEquals(doc.querySelectorAll("#diff-content .hunk-header").length, 1);
});

Deno.test("combined diff: two selected files render two labelled sections in order", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile), structuredClone(sampleFileB)];
  state.commentsCache.set("a.go", []);
  state.commentsCache.set("b.go", []);
  state.selectedFiles = ["a.go", "b.go"];

  renderCombinedDiff(["a.go", "b.go"], {
    actions: noopActions,
    onAddComment: () => {},
  });

  const sections = Array.from(
    doc.querySelectorAll("#diff-content .diff-file-section"),
  );
  assertEquals(sections.length, 2);

  // sections appear in selection order, each tagged with its file and labelled.
  assertEquals(
    sections.map((s) => (s as unknown as HTMLElement).dataset.file),
    ["a.go", "b.go"],
  );
  assertEquals(
    sections.map((s) =>
      s.querySelector(".diff-file-section-name")?.textContent
    ),
    ["a.go", "b.go"],
  );

  // each section carries its own file's hunk.
  for (const section of sections) {
    assert(section.querySelector(".hunk-header"));
  }
});

Deno.test("combined diff: a mutation on one file's line patches only that file's section", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile), structuredClone(sampleFileB)];
  state.commentsCache.set("a.go", []);
  state.commentsCache.set("b.go", []);
  state.selectedFiles = ["a.go", "b.go"];

  renderCombinedDiff(["a.go", "b.go"], {
    actions: noopActions,
    onAddComment: () => {},
  });

  // a comment lands on line 3 of b.go — a line number that also exists in a.go.
  applyMutation({
    file_path: "b.go",
    line_number: 3,
    comments: [comment({ id: "x", anchors: [goodAnchor(3)], content: "in b" })],
    file_status: "active",
  }, { actions: noopActions, rebuildOverview: () => {} });

  const sectionA = doc.querySelector('.diff-file-section[data-file="a.go"]');
  const sectionB = doc.querySelector('.diff-file-section[data-file="b.go"]');
  assert(sectionA);
  assert(sectionB);

  // only b.go's section gains the thread and the commented marker.
  assert(sectionB.querySelector('.comment-thread[data-line="3"]'));
  assert(
    (sectionB.querySelector(
      '.diff-line[data-line="3"]',
    ) as unknown as HTMLElement).classList.contains("commented"),
  );
  assertEquals(
    sectionA.querySelectorAll('.comment-thread[data-line="3"]').length,
    0,
  );
  assert(
    !(sectionA.querySelector(
      '.diff-line[data-line="3"]',
    ) as unknown as HTMLElement).classList.contains("commented"),
  );
});

Deno.test("outdatedCommentsBlock: one item per outdated root with its last context", () => {
  setupDom();
  const context = ["line one", "line two", "line three"];
  const outdated = comment({
    id: "x",
    content: "stale note",
    anchors: [goodAnchor(3, context), adriftAnchor()],
  });
  const live = comment({
    id: "y",
    content: "live note",
    anchors: [goodAnchor(5)],
  });
  const comments = [outdated, live];

  const block = outdatedCommentsBlock("a.go", comments, noopActions);
  assert(block, "expected a block when a root comment is outdated");
  assertEquals(block.getAttribute("data-outdated"), "a.go");

  const items = block.querySelectorAll(".outdated-comment");
  assertEquals(items.length, 1);
  const item = items[0] as unknown as HTMLElement;
  assertEquals(item.getAttribute("data-comment"), "x");

  // the pseudo-hunk has one outdated-line per lastGoodContext line.
  const expectedContext = lastGoodContext(outdated);
  assertEquals(expectedContext, context);
  const hunk = item.querySelector(".outdated-hunk");
  assert(hunk, "expected an outdated-hunk");
  const lines = hunk.querySelectorAll(".outdated-line");
  assertEquals(lines.length, expectedContext.length);
  assertEquals(
    Array.from(lines).map((l) => l.textContent),
    expectedContext,
  );

  // each outdated line also carries the diff-line class.
  assert((lines[0] as unknown as HTMLElement).classList.contains("diff-line"));

  // the comment's content is rendered in the thread.
  assert(item.textContent?.includes("stale note"));
});

Deno.test("outdatedCommentsBlock: null when no root comment is outdated", () => {
  setupDom();
  const comments = [
    comment({ id: "a", anchors: [goodAnchor(3)] }),
    comment({ id: "b", parent_id: "a", content: "reply" }),
  ];
  assertEquals(outdatedCommentsBlock("a.go", comments, noopActions), null);
});

Deno.test("diff: a binary file renders a placeholder, no lines", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "img.png", Hunks: [], Binary: true }];
  state.commentsCache.set("img.png", []);

  renderDiff("img.png", { actions: noopActions, onAddComment: () => {} });

  const placeholder = doc.querySelector("#diff-content .binary-placeholder");
  assert(placeholder);
  assertEquals(placeholder.textContent, "binary file, cannot diff");
  assertEquals(doc.querySelectorAll("#diff-content .diff-line").length, 0);
});
