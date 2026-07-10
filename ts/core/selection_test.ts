import { assertEquals } from "@std/assert";
import { nextSelection, primaryFile } from "./selection.ts";

Deno.test("nextSelection: a plain click replaces the whole selection with one path", () => {
  const given = ["a.go", "b.go"];
  const actual = nextSelection(given, "c.go", false);
  assertEquals(actual, ["c.go"]);
});

Deno.test("nextSelection: a plain re-click of a multi-selection collapses to that one path", () => {
  const given = ["a.go", "b.go"];
  const actual = nextSelection(given, "a.go", false);
  assertEquals(actual, ["a.go"]);
});

Deno.test("nextSelection: an additive click appends an unselected path, preserving order", () => {
  const given = ["a.go"];
  const actual = nextSelection(given, "b.go", true);
  assertEquals(actual, ["a.go", "b.go"]);
});

Deno.test("nextSelection: an additive click removes an already-selected path, keeping the rest", () => {
  const given = ["a.go", "b.go", "c.go"];
  const actual = nextSelection(given, "b.go", true);
  assertEquals(actual, ["a.go", "c.go"]);
});

Deno.test("nextSelection: an additive click on the only selected path is a no-op", () => {
  const given = ["a.go"];
  const actual = nextSelection(given, "a.go", true);
  assertEquals(actual, ["a.go"]);
});

Deno.test("nextSelection: an additive click on an empty selection adds the path", () => {
  const given: string[] = [];
  const actual = nextSelection(given, "a.go", true);
  assertEquals(actual, ["a.go"]);
});

Deno.test("nextSelection: does not mutate the input selection", () => {
  const given = ["a.go", "b.go"];
  nextSelection(given, "c.go", true);
  assertEquals(given, ["a.go", "b.go"]);
});

Deno.test("primaryFile: the last-toggled path is the primary", () => {
  assertEquals(primaryFile(["a.go", "b.go"]), "b.go");
});

Deno.test("primaryFile: an empty selection has no primary", () => {
  assertEquals(primaryFile([]), "");
});
