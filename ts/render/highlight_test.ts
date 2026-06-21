// tests for OUR highlighting logic — the path→language selector and the
// plain-text fallback. We do not test highlight.js's tokenising output; that is
// the library's concern. Run with --allow-env (highlight.js reads env at init).

import { assertEquals, assertStringIncludes } from "@std/assert";
import { highlightLine, languageForPath } from "./highlight.ts";

Deno.test("languageForPath: maps known extensions to grammars", () => {
  const cases: Array<[string, string]> = [
    ["main.go", "go"],
    ["manage.sh", "bash"],
    ["deno.json", "json"],
    ["config.yaml", "yaml"],
    ["a/b/notes.md", "markdown"],
    ["core.clj", "clojure"],
    ["index.html", "xml"],
    ["style.css", "css"],
    ["q.sql", "sql"],
    ["client.ts", "typescript"],
    ["main.js", "javascript"],
  ];
  for (const [path, expected] of cases) {
    assertEquals(languageForPath(path), expected, path);
  }
});

Deno.test("languageForPath: extension match is case-insensitive", () => {
  assertEquals(languageForPath("README.MD"), "markdown");
});

Deno.test("languageForPath: unrecognised extension yields empty (plain text)", () => {
  assertEquals(languageForPath("data.janet"), "");
  assertEquals(languageForPath("schema.cue"), "");
});

Deno.test("languageForPath: a path with no extension yields empty", () => {
  assertEquals(languageForPath("Makefile"), "");
});

Deno.test("highlightLine: unrecognised language returns escaped plain text", () => {
  const given = "a < b && c > d";
  const actual = highlightLine(given, "");
  // our fallback escapes; it does not pass raw markup through.
  assertEquals(actual, "a &lt; b &amp;&amp; c &gt; d");
});

Deno.test("highlightLine: a recognised language produces highlight markup", () => {
  // we assert only that SOME hljs markup is emitted and the line did not throw —
  // not the specific tokenisation, which is the library's behaviour, not ours.
  const actual = highlightLine("func main() {}", "go");
  assertStringIncludes(actual, "hljs-");
});
