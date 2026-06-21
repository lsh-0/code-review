// DOM integration tests: render known state into a real (deno-dom) tree and
// assert the produced structure — nodes, classes, attributes, ordering — rather
// than markup strings. This exercises the render path that pure-core tests
// cannot. Run with --allow-read --allow-net (deno-dom WASM init). It asserts our
// rendering produces the right tree, NOT that WebKit paints it (renderer quirks
// stay a manual concern).

import { assert, assertEquals } from "@std/assert";
import { setupDom } from "./dom_test_setup.ts";
import { renderFileList } from "./filelist.ts";
import { renderDiff } from "./hunks.ts";
import { type CommentActions } from "./comments.ts";
import { state } from "./state.ts";
import type { Comment, DiffFile } from "../core/types.ts";
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
    line_number: 0,
    status: "active",
    context_before: "",
    context_line: "",
    context_after: "",
    ...over,
  };
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
    comment({ status: "active", line_number: 1 }),
  ]);

  renderFileList(noopFileListCb);

  const item = doc.querySelector("#files .file-item");
  assert(item);
  assert(
    (item as unknown as HTMLElement).classList.contains("has-comments-active"),
  );
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
    comment({ id: "x", line_number: 3, content: "look here" }),
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

Deno.test("diff: a binary file renders a placeholder, no lines", () => {
  const doc = setupDom();
  state.diffFiles = [{ Path: "img.png", Hunks: [], Binary: true }];
  state.commentsCache.set("img.png", []);

  renderDiff("img.png", { actions: noopActions, onAddComment: () => {} });

  const placeholder = doc.querySelector("#diff-content .binary-placeholder");
  assert(placeholder);
  assertEquals(doc.querySelectorAll("#diff-content .diff-line").length, 0);
});
