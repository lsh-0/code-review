import { assertEquals } from "@std/assert";
import { grouperFor, groupFiles } from "./grouping.ts";
import type { DiffFileMeta } from "./types.ts";

const files: DiffFileMeta[] = [
  { Path: "a.go", Binary: false },
  { Path: "b.go", Binary: false },
  { Path: "c.go", Binary: false },
];

const noMarks = { isMarked: () => false };

Deno.test("groupFiles none yields one unheaded group of every file in order", () => {
  const actual = groupFiles(files, "none", noMarks);
  assertEquals(actual.length, 1);
  assertEquals(actual[0].label, "");
  assertEquals(actual[0].files.map((f) => f.Path), ["a.go", "b.go", "c.go"]);
});

Deno.test("groupFiles marked puts unmarked first, marked second", () => {
  const marked = new Set(["b.go"]);
  const ctx = { isMarked: (p: string) => marked.has(p) };
  const actual = groupFiles(files, "marked", ctx);

  assertEquals(actual.map((g) => g.label), ["Unmarked", "Marked"]);
  assertEquals(actual[0].files.map((f) => f.Path), ["a.go", "c.go"]);
  assertEquals(actual[1].files.map((f) => f.Path), ["b.go"]);
});

Deno.test("groupFiles marked drops an empty group", () => {
  const actual = groupFiles(files, "marked", noMarks);
  assertEquals(actual.length, 1);
  assertEquals(actual[0].label, "Unmarked");
});

Deno.test("grouperFor falls back to none for an unknown key", () => {
  const given = "no-such-grouper";
  const actual = grouperFor(given);
  assertEquals(actual.key, "none");
});
