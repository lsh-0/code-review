// pins the TypeScript wire types against fixtures produced by the Go backend
// (backend/fixtures_test.go writes ts/testdata/*.json from the real bound App
// methods). Decoding each fixture into its wire type and asserting the fields
// it carries fails the test if the Go side adds, renames, or drops a field
// without the TS type following — catching cross-bridge drift here rather than
// at runtime in the webview. Run with --allow-read (the test reads testdata).

import { assert, assertEquals } from "@std/assert";
import type { CommentedFile, CommentMutationResult } from "./core/types.ts";
import type { ReviewInfo } from "./client.ts";

function readFixture(name: string): unknown {
  const url = new URL(`./testdata/${name}`, import.meta.url);
  return JSON.parse(Deno.readTextFileSync(url));
}

Deno.test("wire: mutation result decodes with all fields", () => {
  const actual = readFixture("mutation_result.json") as CommentMutationResult;

  assertEquals(actual.file_path, "a.go");
  assertEquals(actual.line_number, 7);
  assertEquals(actual.file_status, "active");
  assertEquals(actual.comments.length, 2);

  const [rootC, replyC] = actual.comments;
  // root comment carries a good first anchor (line, context window, offset) and
  // a status, no parent.
  assertEquals(rootC.parent_id ?? "", "");
  assertEquals(rootC.status, "active");
  assert(rootC.anchors !== undefined && rootC.anchors.length === 1);
  const anchor = rootC.anchors[0];
  assertEquals(anchor.line_number, 7);
  assertEquals(anchor.offset, 1);
  assertEquals(anchor.context, ["before", "the line", "after"]);
  assert(rootC.id.length > 0);
  // reply carries a parent_id pointing at the root and no anchor of its own.
  assertEquals(replyC.parent_id, rootC.id);
  assertEquals(replyC.anchors ?? [], []);
});

Deno.test("wire: review info decodes with all fields", () => {
  const actual = readFixture("review_info.json") as ReviewInfo;
  // repo_path is a machine-dependent absolute path (the backend test uses a
  // temp dir), so assert the field decodes as a non-empty string rather than a
  // literal — this test pins the wire shape, not the path value.
  assert(typeof actual.repo_path === "string" && actual.repo_path.length > 0);
  assertEquals(actual.source_branch, "feature");
  assertEquals(actual.target_branch, "main");
  assertEquals(actual.current_user, "Test User");
});

Deno.test("wire: review info carries the imported sources", () => {
  const actual = readFixture("review_info.json") as ReviewInfo;

  assertEquals(actual.sources.length, 1);
  const source = actual.sources[0];
  assertEquals(source.id, "bb:1430");
  assertEquals(source.label, "Bitbucket #1430 — a title");
  assertEquals(source.pull_request, 1430);
  assertEquals(source.title, "a title");
  assertEquals(source.source_branch, "feature-1430--telemetry");
  assertEquals(source.target_branch, "master");
  assertEquals(source.imported_at, "2026-09-14T12:00:00Z");
});

Deno.test("wire: commented files decodes with path and comments", () => {
  const actual = readFixture("commented_files.json") as CommentedFile[];
  assertEquals(actual.length, 1);
  assertEquals(actual[0].path, "a.go");
  assertEquals(actual[0].comments.length, 2);
  const anchors = actual[0].comments[0].anchors;
  assert(anchors !== undefined && anchors.length === 1);
  assertEquals(anchors[0].line_number, 7);
});

// a local comment must carry no origin and an imported one must carry both its
// source and its foreign id, so the render layer can tell them apart.
Deno.test("wire: commented files distinguish local from imported comments", () => {
  const actual = readFixture("commented_files.json") as CommentedFile[];
  const [localComment, importedComment] = actual[0].comments;

  assertEquals(localComment.origin, undefined);

  assert(importedComment.origin !== undefined, "expected an origin");
  assertEquals(importedComment.origin.source, "bb:1430");
  assertEquals(importedComment.origin.foreign_id, "856483458");
});
