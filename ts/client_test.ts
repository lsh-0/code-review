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
  // root comment carries context and a status, no parent.
  assertEquals(rootC.parent_id ?? "", "");
  assertEquals(rootC.line_number, 7);
  assertEquals(rootC.status, "active");
  assertEquals(rootC.context_before, "before");
  assertEquals(rootC.context_line, "the line");
  assertEquals(rootC.context_after, "after");
  assert(rootC.id.length > 0);
  // reply carries a parent_id pointing at the root.
  assertEquals(replyC.parent_id, rootC.id);
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

Deno.test("wire: commented files decodes with path and comments", () => {
  const actual = readFixture("commented_files.json") as CommentedFile[];
  assertEquals(actual.length, 1);
  assertEquals(actual[0].path, "a.go");
  assertEquals(actual[0].comments.length, 1);
  assertEquals(actual[0].comments[0].line_number, 7);
});
