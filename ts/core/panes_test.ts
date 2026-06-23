import { assertEquals } from "@std/assert";
import { clampPaneWidth } from "./panes.ts";

Deno.test("clampPaneWidth leaves an in-range width unchanged", () => {
  const given = 300;
  const expected = 300;
  const actual = clampPaneWidth(given, 1000);
  assertEquals(actual, expected);
});

Deno.test("clampPaneWidth clamps below the 5% minimum up to the floor", () => {
  const given = 10;
  const expected = 50; // 5% of 1000
  const actual = clampPaneWidth(given, 1000);
  assertEquals(actual, expected);
});

Deno.test("clampPaneWidth clamps above the 95% maximum down to the ceiling", () => {
  const given = 990;
  const expected = 950; // 95% of 1000
  const actual = clampPaneWidth(given, 1000);
  assertEquals(actual, expected);
});

Deno.test("clampPaneWidth scales the bounds with the window width", () => {
  const given = 10;
  const expected = 100; // 5% of 2000
  const actual = clampPaneWidth(given, 2000);
  assertEquals(actual, expected);
});
