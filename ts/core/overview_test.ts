import { assertEquals } from "@std/assert";
import {
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

Deno.test("getLineContext: returns before, line, after within a hunk", () => {
  const given = file([hunk([line(1, "a"), line(2, "b"), line(3, "c")])]);
  const actual = getLineContext(given, 2);
  assertEquals(actual, { before: "a", line: "b", after: "c" });
});

Deno.test("getLineContext: first line has no before", () => {
  const given = file([hunk([line(1, "a"), line(2, "b")])]);
  const actual = getLineContext(given, 1);
  assertEquals(actual, { before: "", line: "a", after: "b" });
});

Deno.test("getLineContext: last line has no after", () => {
  const given = file([hunk([line(1, "a"), line(2, "b")])]);
  const actual = getLineContext(given, 2);
  assertEquals(actual, { before: "a", line: "b", after: "" });
});

Deno.test("getLineContext: missing file returns empties", () => {
  const actual = getLineContext(undefined, 2);
  assertEquals(actual, { before: "", line: "", after: "" });
});

Deno.test("getLineContext: line not found returns empties", () => {
  const given = file([hunk([line(1, "a")])]);
  const actual = getLineContext(given, 99);
  assertEquals(actual, { before: "", line: "", after: "" });
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
