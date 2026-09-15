// DOM integration tests: render known state into a real (deno-dom) tree and
// assert the produced structure — nodes, classes, attributes, ordering — rather
// than markup strings. This exercises the render path that pure-core tests
// cannot: rendering is a function of state, so each test seeds `state` and
// asserts the tree. Run with --allow-read --allow-net (deno-dom WASM init). It
// asserts our rendering produces the right tree, NOT that WebKit paints it
// (renderer quirks stay a manual concern).

import { assert, assertEquals } from "@std/assert";
import { setupDom } from "./dom_test_setup.ts";
import { renderFileList } from "./filelist.ts";
import { renderCombinedDiff, renderDiff, renderFileHunks } from "./hunks.ts";
import { applyMutation } from "./mutate.ts";
import { outdatedCommentsBlock } from "./comments.ts";
import { rerenderFileRegion } from "./view.ts";
import { closeFind, openFind } from "./find.ts";
import { type FileViewState, state } from "./state.ts";
import type { Comment, DiffFile } from "../core/types.ts";
import { LineAdded, LineContext } from "../core/types.ts";
import { defaultWindows } from "../core/windows.ts";

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
    line: 0,
    outdated: false,
    ...over,
  };
}

// a ready-made file view: windows computed from the given file and comment
// lines, with the context lines in `lines` pre-fetched — so rendering needs no
// bridge and the test is purely state-in/tree-out.
function seedFileView(
  file: DiffFile,
  commentLines: number[],
  totalLines: number,
  lines: Record<number, string> = {},
): FileViewState {
  const view: FileViewState = {
    windows: defaultWindows(file.Hunks, commentLines, totalLines),
    fileLines: new Map(
      Object.entries(lines).map(([n, c]) => [Number(n), c]),
    ),
    totalLines,
    fetchFailed: false,
    fetching: false,
  };
  state.fileViews.set(file.Path, view);
  return view;
}

Deno.test("file list: renders one item per diff file in order, with paths", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "a.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
    { Path: "b.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
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
  state.diffFiles = [
    { Path: "a.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
  ];
  state.commentsCache.set("a.go", [
    comment({ status: "active", line: 1 }),
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
    { Path: "a.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
    { Path: "b.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
    { Path: "c.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
  ];
  state.selectedFiles = ["a.go", "c.go"];

  renderFileList(noopFileListCb);

  const active = Array.from(doc.querySelectorAll("#files .file-item.active"))
    .map((i) => (i as unknown as HTMLElement).dataset.path);
  assertEquals(active, ["a.go", "c.go"]);
});

Deno.test("file list: a marked file renders a checked checkbox", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "a.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
  ];
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
    {
      Path: "ts/render/modals.ts",
      Hunks: [],
      Binary: false,
      Renamed: false,
      OldPath: "",
    },
    {
      Path: "backend/main.go",
      Hunks: [],
      Binary: false,
      Renamed: false,
      OldPath: "",
    },
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
  state.diffFiles = [
    {
      Path: "backend/main.go",
      Hunks: [],
      Binary: false,
      Renamed: false,
      OldPath: "",
    },
  ];
  state.fileFilter = "no-such-file";

  renderFileList(noopFileListCb);

  assertEquals(doc.querySelectorAll("#files .file-item").length, 0);
  assert(doc.querySelector("#overview-footer .overview-item"));
});

Deno.test("file list: grouping by mark renders headings, unmarked group first", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "a.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
    { Path: "b.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
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
  state.diffFiles = [
    { Path: "a.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
  ];

  renderFileList(noopFileListCb);

  assertEquals(doc.querySelectorAll("#files .file-group-heading").length, 0);
});

Deno.test("file list: the overview footer entry is always rendered", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "a.go", Hunks: [], Binary: false, Renamed: false, OldPath: "" },
  ];
  renderFileList(noopFileListCb);
  const entry = doc.querySelector("#overview-footer .overview-item");
  assert(entry);
  assertEquals(entry.textContent, "Review overview");
});

const sampleFile: DiffFile = {
  Path: "a.go",
  Binary: false,
  Renamed: false,
  OldPath: "",
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
  state.selectedFiles = ["a.go"];

  renderDiff("a.go");

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

  renderDiff("a.go");

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
    comment({ id: "x", line: 3, content: "look here" }),
  ]);

  renderDiff("a.go");

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

Deno.test("windows: the sample hunk renders with both boundary affordances disabled", () => {
  const doc = setupDom();
  // the hunk starts at line 1 (nothing above) and its short trailing context
  // says it reached end-of-file (nothing below): both rows present, disabled.
  state.diffFiles = [structuredClone(sampleFile)];
  state.commentsCache.set("a.go", []);

  renderDiff("a.go");

  const rows = doc.querySelectorAll("#diff-content .expand-row");
  assertEquals(rows.length, 2);
  for (const row of rows) {
    assert(
      (row as unknown as HTMLElement).classList.contains("disabled"),
      "expected the boundary affordances disabled",
    );
  }
});

// a file whose single hunk sits mid-file, so both directions have hidden lines.
const midFile: DiffFile = {
  Path: "mid.go",
  Binary: false,
  Renamed: false,
  OldPath: "",
  Hunks: [
    {
      OldStart: 10,
      OldLines: 3,
      NewStart: 10,
      NewLines: 4,
      Lines: [
        { Type: LineContext, Content: "ctx 10", OldLineNo: 10, NewLineNo: 10 },
        { Type: LineAdded, Content: "added 11", OldLineNo: 0, NewLineNo: 11 },
        { Type: LineContext, Content: "ctx 12", OldLineNo: 11, NewLineNo: 12 },
        { Type: LineContext, Content: "ctx 13", OldLineNo: 12, NewLineNo: 13 },
      ],
    },
  ],
};

Deno.test("windows: a comment outside the hunk renders as its own island with its thread", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(midFile)];
  state.commentsCache.set("mid.go", [
    comment({ id: "x", line: 25, content: "tail note" }),
  ]);
  // the island [22..28] pre-fetched, so the render is pure state-in/tree-out.
  seedFileView(midFile, [25], 40, {
    22: "l22",
    23: "l23",
    24: "l24",
    25: "l25",
    26: "l26",
    27: "l27",
    28: "l28",
  });

  renderDiff("mid.go");

  // two windows render: the hunk and the island.
  assertEquals(doc.querySelectorAll("#diff-content .diff-hunk").length, 2);

  // the island's lines are ordinary context rows with correct numbering.
  const islandRow = doc.querySelector(
    '#diff-content .diff-line[data-line="25"]',
  ) as unknown as HTMLElement;
  assert(islandRow, "expected the commented island line to render");
  assert(islandRow.textContent?.includes("l25"));

  // the comment thread sits against its line inside the island, not untethered.
  const thread = doc.querySelector(
    '#diff-content .comment-thread[data-line="25"]',
  );
  assert(thread, "expected the thread against the island line");
  assert(thread.textContent?.includes("tail note"));
  assertEquals(
    doc.querySelectorAll("#diff-content .outdated-comments").length,
    0,
  );

  // the island does not run to the hunk or the file edges: line 20 is hidden.
  assertEquals(
    doc.querySelector('#diff-content .diff-line[data-line="20"]'),
    null,
  );
});

Deno.test("windows: a grown window renders its revealed context from state", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(midFile)];
  state.commentsCache.set("mid.go", []);
  // a view grown upward beyond the hunk: window [5..13] with lines fetched.
  const view = seedFileView(midFile, [], 40, {
    5: "l5",
    6: "l6",
    7: "l7",
    8: "l8",
    9: "l9",
  });
  view.windows = [{ start: 5, end: 13 }];

  renderDiff("mid.go");

  // revealed context rows precede the hunk header's lines, numbered correctly.
  const row = doc.querySelector(
    '#diff-content .diff-line[data-line="5"]',
  ) as unknown as HTMLElement;
  assert(row, "expected the revealed line to render from state");
  assert(row.textContent?.includes("l5"));
  // the hunk's own lines still render.
  assert(doc.querySelector('#diff-content .diff-line[data-line="11"]'));
});

Deno.test("windows: two hunks in one merged window hide the lower header", () => {
  const doc = setupDom();
  // two hunks separated by a single hidden line (14): the default windows merge.
  const file: DiffFile = {
    Path: "m.go",
    Binary: false,
    Renamed: false,
    OldPath: "",
    Hunks: [
      structuredClone(midFile.Hunks[0]),
      {
        OldStart: 14,
        OldLines: 2,
        NewStart: 15,
        NewLines: 2,
        Lines: [
          {
            Type: LineContext,
            Content: "ctx 15",
            OldLineNo: 14,
            NewLineNo: 15,
          },
          { Type: LineAdded, Content: "added 16", OldLineNo: 0, NewLineNo: 16 },
        ],
      },
    ],
  };
  state.diffFiles = [file];
  state.commentsCache.set("m.go", []);
  seedFileView(file, [], 40, { 14: "l14" });

  renderDiff("m.go");

  // one window: one .diff-hunk container with both hunk headers, lower hidden.
  assertEquals(doc.querySelectorAll("#diff-content .diff-hunk").length, 1);
  const headers = doc.querySelectorAll("#diff-content .hunk-header");
  assertEquals(headers.length, 2);
  assert(
    !(headers[0] as unknown as HTMLElement).classList.contains("joined-hidden"),
  );
  assert(
    (headers[1] as unknown as HTMLElement).classList.contains("joined-hidden"),
  );

  // the single hidden line between them renders as context.
  assert(doc.querySelector('#diff-content .diff-line[data-line="14"]'));
});

Deno.test("windows: an unplaceable anchored comment falls back to the untethered block", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(midFile)];
  state.commentsCache.set("mid.go", [
    comment({
      id: "x",
      line: 99,
      content: "beyond the end",
      context: ["was here"],
    }),
  ]);
  // total is known (40), so the line-99 island is clamped away entirely.
  seedFileView(midFile, [99], 40);

  renderDiff("mid.go");

  const block = doc.querySelector("#diff-content .outdated-comments");
  assert(block, "expected the untethered block for an unshowable comment");
  assert(block.textContent?.includes("beyond the end"));
  assert(block.textContent?.includes("was here"));
});

// a second file whose single hunk shares line number 3 with `sampleFile`, so a
// combined view of both stacks two sections that each carry a `data-line="3"`.
const sampleFileB: DiffFile = {
  Path: "b.go",
  Binary: false,
  Renamed: false,
  OldPath: "",
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

  renderCombinedDiff(["a.go"]);

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

  renderCombinedDiff(["a.go", "b.go"]);

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

Deno.test("combined diff: a mutation on one file re-renders only that file's section", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile), structuredClone(sampleFileB)];
  state.commentsCache.set("a.go", []);
  state.commentsCache.set("b.go", []);
  state.selectedFiles = ["a.go", "b.go"];

  renderCombinedDiff(["a.go", "b.go"]);

  // a comment lands on line 3 of b.go — a line number that also exists in a.go.
  applyMutation({
    file_path: "b.go",
    line_number: 3,
    comments: [comment({ id: "x", line: 3, content: "in b" })],
    file_status: "active",
  });

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

Deno.test("mutation: a re-render preserves the file's grown window state", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(midFile)];
  state.commentsCache.set("mid.go", []);
  state.selectedFiles = ["mid.go"];
  // grown upward: revealed lines 7..9 above the hunk.
  const view = seedFileView(midFile, [], 40, { 7: "l7", 8: "l8", 9: "l9" });
  view.windows = [{ start: 7, end: 13 }];

  renderDiff("mid.go");
  assert(doc.querySelector('#diff-content .diff-line[data-line="7"]'));

  // a comment mutation on the revealed line re-renders the region from state.
  applyMutation({
    file_path: "mid.go",
    line_number: 8,
    comments: [comment({ id: "x", line: 8, content: "on revealed" })],
    file_status: "active",
  });

  // the grown context is still shown (window state survived the re-render)
  // and the new thread sits against its revealed line.
  assert(
    doc.querySelector('#diff-content .diff-line[data-line="7"]'),
    "expected the grown window to survive the mutation re-render",
  );
  const thread = doc.querySelector(
    '#diff-content .comment-thread[data-line="8"]',
  );
  assert(thread, "expected the thread against the revealed line");
  assert(thread.textContent?.includes("on revealed"));
});

Deno.test("regions: re-rendering a file that is no longer shown changes nothing", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile), structuredClone(sampleFileB)];
  state.commentsCache.set("a.go", []);
  state.commentsCache.set("b.go", []);
  state.selectedFiles = ["b.go"];

  renderDiff("b.go");
  const before = (doc.querySelector("#diff-content") as unknown as HTMLElement)
    .innerHTML;

  // a late async result for a.go (navigated away): its region render is a no-op.
  rerenderFileRegion("a.go");

  const after = (doc.querySelector("#diff-content") as unknown as HTMLElement)
    .innerHTML;
  assertEquals(after, before);
});

Deno.test("find: highlights are re-applied after a mutation re-renders the region", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile)];
  state.commentsCache.set("a.go", []);
  state.selectedFiles = ["a.go"];
  renderDiff("a.go");

  openFind();
  const input = doc.querySelector("#find-input") as unknown as HTMLInputElement;
  assert(input, "expected the find bar input");
  input.value = "main";
  input.dispatchEvent(new Event("input"));
  assert(
    doc.querySelectorAll(".find-match").length > 0,
    "expected matches before the mutation",
  );

  // the mutation replaces the region's DOM; the highlights must come back.
  applyMutation({
    file_path: "a.go",
    line_number: 3,
    comments: [comment({ id: "x", line: 3, content: "note" })],
    file_status: "active",
  });

  assert(
    doc.querySelectorAll("#diff-content .find-match").length > 0,
    "expected highlights re-applied to the re-rendered region",
  );
  closeFind();
});

Deno.test("outdatedCommentsBlock: one item per outdated root with its last context", () => {
  setupDom();
  const context = ["line one", "line two", "line three"];
  const outdated = comment({
    id: "x",
    content: "stale note",
    outdated: true,
    context,
  });
  const live = comment({
    id: "y",
    content: "live note",
    line: 5,
  });
  const comments = [outdated, live];

  const block = outdatedCommentsBlock("a.go", comments);
  assert(block, "expected a block when a root comment is outdated");
  assertEquals(block.getAttribute("data-outdated"), "a.go");

  const items = block.querySelectorAll(".outdated-comment");
  assertEquals(items.length, 1);
  const item = items[0] as unknown as HTMLElement;
  assertEquals(item.getAttribute("data-comment"), "x");

  // the pseudo-hunk has one outdated-line per stored context line.
  const hunk = item.querySelector(".outdated-hunk");
  assert(hunk, "expected an outdated-hunk");
  const lines = hunk.querySelectorAll(".outdated-line");
  assertEquals(lines.length, context.length);
  assertEquals(
    Array.from(lines).map((l) => l.textContent),
    context,
  );

  // each outdated line also carries the diff-line class.
  assert((lines[0] as unknown as HTMLElement).classList.contains("diff-line"));

  // the comment's content is rendered in the thread.
  assert(item.textContent?.includes("stale note"));
});

Deno.test("outdatedCommentsBlock: null when no root comment is outdated", () => {
  setupDom();
  const comments = [
    comment({ id: "a", line: 3 }),
    comment({ id: "b", parent_id: "a", content: "reply" }),
  ];
  assertEquals(outdatedCommentsBlock("a.go", comments), null);
});

Deno.test("diff: a binary file renders a placeholder, no lines", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "img.png", Hunks: [], Binary: true, Renamed: false, OldPath: "" },
  ];
  state.commentsCache.set("img.png", []);

  renderDiff("img.png");

  const placeholder = doc.querySelector("#diff-content .binary-placeholder");
  assert(placeholder);
  assertEquals(placeholder.textContent, "binary file, cannot diff");
  assertEquals(doc.querySelectorAll("#diff-content .diff-line").length, 0);
});

Deno.test("diff: a pure rename renders a placeholder naming the old path", () => {
  const doc = setupDom();
  state.diffFiles = [
    {
      Path: "new.go",
      Hunks: [],
      Binary: false,
      Renamed: true,
      OldPath: "old.go",
    },
  ];
  state.commentsCache.set("new.go", []);

  renderDiff("new.go");

  const placeholder = doc.querySelector("#diff-content .rename-placeholder");
  assert(placeholder);
  assertEquals(placeholder.textContent, "renamed from old.go, no changes");
  assertEquals(doc.querySelectorAll("#diff-content .diff-line").length, 0);
});

Deno.test("diff: a rename with edits renders its hunks, not the placeholder", () => {
  const doc = setupDom();
  state.diffFiles = [{
    ...structuredClone(sampleFile),
    Path: "new.go",
    Renamed: true,
    OldPath: "old.go",
  }];
  state.commentsCache.set("new.go", []);

  renderDiff("new.go");

  assertEquals(
    doc.querySelectorAll("#diff-content .rename-placeholder").length,
    0,
  );
  assert(doc.querySelectorAll("#diff-content .diff-line").length > 0);
});

Deno.test("diff: the single-file view shows a top-of-file 'Add comment' control", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile)];
  state.commentsCache.set("a.go", []);

  renderDiff("a.go");

  const block = doc.querySelector("#diff-content .top-level-comments");
  assert(block, "expected a top-level block at the top of the file");
  assertEquals(block.getAttribute("data-toplevel"), "a.go");
  const button = block.querySelector(".top-level-add-comment");
  assert(button, "expected an 'Add comment' control");
  assertEquals(button.textContent, "Add comment");

  // the top-level block is the first child of the content, above the code.
  const content = doc.querySelector("#diff-content");
  assertEquals(content?.firstElementChild, block);
});

Deno.test("diff: a top-level comment renders at the top, before untethered and code", () => {
  const doc = setupDom();
  state.diffFiles = [structuredClone(sampleFile)];
  state.commentsCache.set("a.go", [
    // a top-level (anchorless) comment, an outdated one, and a live line comment.
    comment({ id: "top", content: "whole-file note" }),
    comment({
      id: "old",
      outdated: true,
      context: ["ctx"],
      content: "gone",
    }),
    comment({ id: "live", line: 3, content: "look here" }),
  ]);

  renderDiff("a.go");

  const content = doc.querySelector("#diff-content");
  const topLevel = content?.querySelector(".top-level-comments");
  const outdated = content?.querySelector(".outdated-comments");
  const firstHunk = content?.querySelector(".diff-hunk");
  assert(topLevel, "expected a top-level block");
  assert(outdated, "expected an untethered block");
  assert(firstHunk, "expected the code hunks");
  assert(topLevel.textContent?.includes("whole-file note"));

  // DOM order: top-level, then untethered, then code.
  const kids = Array.from(content!.children);
  assert(
    kids.indexOf(topLevel) < kids.indexOf(outdated),
    "top-level must precede untethered",
  );
  assert(
    kids.indexOf(outdated) < kids.indexOf(firstHunk),
    "untethered must precede the code",
  );
});

Deno.test("diff: a binary file offers the control and shows its top-level comment", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "img.png", Hunks: [], Binary: true, Renamed: false, OldPath: "" },
  ];
  state.commentsCache.set("img.png", [
    comment({ id: "b1", content: "wrong asset" }),
  ]);

  renderDiff("img.png");

  const block = doc.querySelector("#diff-content .top-level-comments");
  assert(block, "expected the top-level block on a binary file");
  assert(block.querySelector(".top-level-add-comment"), "expected the control");
  assert(block.textContent?.includes("wrong asset"));

  // the placeholder still shows and no diff lines are rendered.
  assert(doc.querySelector("#diff-content .binary-placeholder"));
  assertEquals(doc.querySelectorAll("#diff-content .diff-line").length, 0);
});

Deno.test("overview: a file's top-level comment shows read-only, no control", () => {
  const doc = setupDom();
  state.diffFiles = [
    { Path: "img.png", Hunks: [], Binary: true, Renamed: false, OldPath: "" },
  ];
  state.commentsCache.set("img.png", [
    comment({ id: "b1", content: "wrong asset" }),
  ]);

  // render the overview-only path for the file, as the overview does. The
  // deno-dom element is cast to the DOM `Element` the render layer is typed
  // against; it is structurally compatible at runtime.
  const section = doc.querySelector("#diff-content") as unknown as HTMLElement;
  renderFileHunks(section, "img.png", true);

  const block = doc.querySelector("#diff-content .top-level-comments");
  assert(block, "expected the top-level block in the overview");
  assert(block.textContent?.includes("wrong asset"));
  assertEquals(
    doc.querySelector("#diff-content .top-level-add-comment"),
    null,
    "overview shows no 'Add comment' control",
  );
});
