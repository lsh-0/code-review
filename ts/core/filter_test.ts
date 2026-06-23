import { assertEquals } from "@std/assert";
import { fileMatchesFilter } from "./filter.ts";

Deno.test("fileMatchesFilter matches a case-insensitive substring of the path", () => {
  const given = "ts/render/modals.ts";
  assertEquals(fileMatchesFilter(given, "modal"), true);
  assertEquals(fileMatchesFilter(given, "MODAL"), true);
  assertEquals(fileMatchesFilter(given, "render/mod"), true);
});

Deno.test("fileMatchesFilter rejects a term that is not a substring", () => {
  const given = "ts/render/modals.ts";
  const actual = fileMatchesFilter(given, "controller");
  assertEquals(actual, false);
});

Deno.test("fileMatchesFilter treats an empty or whitespace term as matching all", () => {
  const given = "backend/main.go";
  assertEquals(fileMatchesFilter(given, ""), true);
  assertEquals(fileMatchesFilter(given, "   "), true);
});

Deno.test("fileMatchesFilter ignores surrounding whitespace in the term", () => {
  const given = "backend/main.go";
  const actual = fileMatchesFilter(given, "  main  ");
  assertEquals(actual, true);
});
