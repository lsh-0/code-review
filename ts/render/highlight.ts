// syntax highlighting at the render boundary. highlight.js is an opaque
// third-party effect layered over the diff for rough visual differentiation;
// its output is not our concern to test. What IS ours — and is unit-tested in
// highlight_test.ts — is the `path → language` selector and the plain-text
// fallback. We import `lib/core` and register only the reviewed languages, so
// the bundle carries a curated set, not highlight.js's full ~190-language build.

import hljs from "highlight.js/lib/core";
import go from "highlight.js/lib/languages/go";
import bash from "highlight.js/lib/languages/bash";
import json from "highlight.js/lib/languages/json";
import yaml from "highlight.js/lib/languages/yaml";
import markdown from "highlight.js/lib/languages/markdown";
import clojure from "highlight.js/lib/languages/clojure";
import xml from "highlight.js/lib/languages/xml";
import css from "highlight.js/lib/languages/css";
import sql from "highlight.js/lib/languages/sql";
import typescript from "highlight.js/lib/languages/typescript";
import javascript from "highlight.js/lib/languages/javascript";

import { escapeHTML } from "../dom.ts";

// the curated languages, registered once. A file whose extension maps to no
// registered language falls back to plain text (see `languageForPath`).
hljs.registerLanguage("go", go);
hljs.registerLanguage("bash", bash);
hljs.registerLanguage("json", json);
hljs.registerLanguage("yaml", yaml);
hljs.registerLanguage("markdown", markdown);
hljs.registerLanguage("clojure", clojure);
hljs.registerLanguage("xml", xml);
hljs.registerLanguage("css", css);
hljs.registerLanguage("sql", sql);
hljs.registerLanguage("typescript", typescript);
hljs.registerLanguage("javascript", javascript);

// the highlight.js language name for a file extension, or "" for an unrecognised
// extension (which renders as plain text). This selector is our logic and is
// unit-tested; the lookup is by the path's final extension, lowercased. Several
// extensions map to one grammar (e.g. .htm/.html/.xml → xml). This is pure.
export function languageForPath(path: string): string {
  const dot = path.lastIndexOf(".");
  if (dot < 0) {
    return "";
  }
  const ext = path.slice(dot + 1).toLowerCase();
  return EXTENSION_LANGUAGE[ext] ?? "";
}

// the extension-to-grammar map is maintained by hand because highlight.js does
// not map file extensions to languages: it keys by language *name* and *alias*,
// not by path. Its only content-based option, `highlightAuto`, is slow and
// unreliable per line and would defeat the curated `lib/core` build chosen to
// keep the bundle small. So a path-to-grammar lookup has to live here.
const EXTENSION_LANGUAGE: Record<string, string> = {
  go: "go",
  sh: "bash",
  bash: "bash",
  zsh: "bash",
  json: "json",
  yaml: "yaml",
  yml: "yaml",
  md: "markdown",
  markdown: "markdown",
  clj: "clojure",
  cljs: "clojure",
  cljc: "clojure",
  edn: "clojure",
  html: "xml",
  htm: "xml",
  xml: "xml",
  svg: "xml",
  css: "css",
  sql: "sql",
  ts: "typescript",
  tsx: "typescript",
  js: "javascript",
  mjs: "javascript",
  jsx: "javascript",
};

// the HTML for one line of code: highlight.js's marked-up output when the
// language is recognised and highlighting succeeds, otherwise the escaped plain
// code. A highlighting error degrades to escaped plain text rather than
// propagating, so a single odd line never breaks the diff (the spec's
// "highlighting failure does not break the diff").
export function highlightLine(code: string, language: string): string {
  if (language === "") {
    return escapeHTML(code);
  }
  try {
    return hljs.highlight(code, { language, ignoreIllegals: true }).value;
  } catch {
    return escapeHTML(code);
  }
}
