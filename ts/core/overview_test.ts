import { assertEquals } from "@std/assert";
import {
  captureContextRadius,
  getLineContext,
  hunkHasComments,
  overviewContextLines,
  overviewVisibleLines,
} from "./overview.ts";
import {
  type DiffFile,
  type DiffHunk,
  type DiffLine,
  LineContext,
} from "./types.ts";

function line(newLineNo: number, content: string): DiffLine {
  return {
    Type: LineContext,
    Content: content,
    OldLineNo: newLineNo,
    NewLineNo: newLineNo,
  };
}

function hunk(lines: DiffLine[]): DiffHunk {
  return {
    OldStart: 1,
    OldLines: lines.length,
    NewStart: 1,
    NewLines: lines.length,
    Lines: lines,
  };
}

function file(hunks: DiffHunk[]): DiffFile {
  return { Path: "f.go", Hunks: hunks, Binary: false };
}

Deno.test("getLineContext: window spans the anchored line and its neighbours", () => {
  // a 3-line hunk fits entirely within the radius-3 window; the anchored line
  // (index 1) sits at offset 1 because the window is clipped at the hunk start.
  const given = file([hunk([line(1, "a"), line(2, "b"), line(3, "c")])]);
  const actual = getLineContext(given, 2);
  assertEquals(actual, { context: ["a", "b", "c"], offset: 1 });
});

Deno.test("getLineContext: first line anchors at offset 0", () => {
  const given = file([hunk([line(1, "a"), line(2, "b")])]);
  const actual = getLineContext(given, 1);
  assertEquals(actual, { context: ["a", "b"], offset: 0 });
});

Deno.test("getLineContext: last line is the final window entry", () => {
  const given = file([hunk([line(1, "a"), line(2, "b")])]);
  const actual = getLineContext(given, 2);
  assertEquals(actual, { context: ["a", "b"], offset: 1 });
});

Deno.test("getLineContext: an interior line gives a full radius window", () => {
  // 9 lines, anchored on line 5 (index 4): the window is the radius-3 span
  // indices 1..7, so 7 lines, with the anchored line at offset 3 (=radius).
  const lines = Array.from({ length: 9 }, (_, i) => line(i + 1, `l${i + 1}`));
  const given = file([hunk(lines)]);
  const actual = getLineContext(given, 5);
  assertEquals(actual, {
    context: ["l2", "l3", "l4", "l5", "l6", "l7", "l8"],
    offset: 3,
  });
  assertEquals(actual.offset, captureContextRadius);
});

Deno.test("getLineContext: window clips at the hunk end", () => {
  // anchored on the last line (index 5) of a 6-line hunk: the window reaches
  // back radius-3 lines (indices 2..5) and the anchored line is the last entry.
  const lines = Array.from({ length: 6 }, (_, i) => line(i + 1, `l${i + 1}`));
  const given = file([hunk(lines)]);
  const actual = getLineContext(given, 6);
  assertEquals(actual, {
    context: ["l3", "l4", "l5", "l6"],
    offset: 3,
  });
});

Deno.test("getLineContext: missing file returns an empty window", () => {
  const actual = getLineContext(undefined, 2);
  assertEquals(actual, { context: [], offset: 0 });
});

Deno.test("getLineContext: line not found returns an empty window", () => {
  const given = file([hunk([line(1, "a")])]);
  const actual = getLineContext(given, 99);
  assertEquals(actual, { context: [], offset: 0 });
});

Deno.test("hunkHasComments: true when a new-side line carries a comment", () => {
  const given = hunk([line(1, "a"), line(2, "b")]);
  const actual = hunkHasComments(given, (n) => n === 2);
  assertEquals(actual, true);
});

Deno.test("hunkHasComments: false when none carry a comment", () => {
  const given = hunk([line(1, "a"), line(2, "b")]);
  const actual = hunkHasComments(given, () => false);
  assertEquals(actual, false);
});

Deno.test("overviewVisibleLines: window is centred, one fewer above than below", () => {
  // 10 lines, comment on line index 5. overviewContextLines=3 ->
  // above: indices 3,4 (overviewContextLines-1=2 lines), below: 6,7,8 (3 lines).
  const lines = Array.from({ length: 10 }, (_, i) => line(i + 1, `l${i + 1}`));
  const given = hunk(lines);
  const actual = overviewVisibleLines(given, (n) => n === 6);
  const expected = [
    false,
    false,
    false,
    true,
    true,
    true,
    true,
    true,
    true,
    false,
  ];
  assertEquals(actual, expected);
  // sanity on the constant the centring relies on.
  assertEquals(overviewContextLines, 3);
});

Deno.test("overviewVisibleLines: window clamps at hunk bounds", () => {
  const lines = [line(1, "a"), line(2, "b"), line(3, "c")];
  const given = hunk(lines);
  // comment on the first line: above clamps to 0, below reaches the end.
  const actual = overviewVisibleLines(given, (n) => n === 1);
  assertEquals(actual, [true, true, true]);
});

Deno.test("overviewVisibleLines: no comments yields all-false", () => {
  const lines = [line(1, "a"), line(2, "b")];
  const actual = overviewVisibleLines(hunk(lines), () => false);
  assertEquals(actual, [false, false]);
});
