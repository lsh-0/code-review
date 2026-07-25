import { assertEquals } from "@std/assert";
import {
  type FindMatch,
  findMatches,
  nextMatchIndex,
  prevMatchIndex,
} from "./find.ts";

Deno.test("findMatches locates every case-insensitive occurrence in order", () => {
  const given = "Foo bar foo BAR";
  const expected: FindMatch[] = [
    { start: 0, length: 3 },
    { start: 8, length: 3 },
  ];
  const actual = findMatches(given, "foo");
  assertEquals(actual, expected);
});

Deno.test("findMatches returns ranges over the original-case string", () => {
  const given = "aXbXc";
  const actual = findMatches(given, "x");
  assertEquals(actual, [
    { start: 1, length: 1 },
    { start: 3, length: 1 },
  ]);
});

Deno.test("findMatches treats an empty or whitespace term as no matches", () => {
  const given = "anything at all";
  assertEquals(findMatches(given, ""), []);
  assertEquals(findMatches(given, "   "), []);
});

Deno.test("findMatches returns no matches when the term is absent", () => {
  const actual = findMatches("hello world", "xyz");
  assertEquals(actual, []);
});

Deno.test("findMatches advances past each match, so overlaps yield the leftmost set", () => {
  const given = "aaaa";
  const actual = findMatches(given, "aa");
  assertEquals(actual, [
    { start: 0, length: 2 },
    { start: 2, length: 2 },
  ]);
});

Deno.test("findMatches trims the term before searching", () => {
  const given = "the main function";
  const actual = findMatches(given, "  main  ");
  assertEquals(actual, [{ start: 4, length: 4 }]);
});

Deno.test("nextMatchIndex advances and wraps past the end to the first", () => {
  assertEquals(nextMatchIndex(0, 3), 1);
  assertEquals(nextMatchIndex(1, 3), 2);
  assertEquals(nextMatchIndex(2, 3), 0);
});

Deno.test("nextMatchIndex returns 0 for an out-of-range or empty selection", () => {
  assertEquals(nextMatchIndex(-1, 3), 0);
  assertEquals(nextMatchIndex(5, 3), 0);
  assertEquals(nextMatchIndex(0, 0), 0);
});

Deno.test("prevMatchIndex retreats and wraps past the start to the last", () => {
  assertEquals(prevMatchIndex(2, 3), 1);
  assertEquals(prevMatchIndex(1, 3), 0);
  assertEquals(prevMatchIndex(0, 3), 2);
});

Deno.test("prevMatchIndex returns the last index for an out-of-range selection", () => {
  assertEquals(prevMatchIndex(-1, 3), 2);
  assertEquals(prevMatchIndex(5, 3), 2);
  assertEquals(prevMatchIndex(0, 0), 0);
});
