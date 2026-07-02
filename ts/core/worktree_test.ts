import { assertEquals } from "@std/assert";
import type { WorkingTreeStatus } from "./types.ts";
import { isFileDirty, workingTreeBannerText } from "./worktree.ts";

// build a status from explicit modified/deleted lists, deriving `dirty_files`
// the way the backend does (the union of both as a set).
function status(
  modified: string[],
  deleted: string[],
): WorkingTreeStatus {
  const dirty_files: Record<string, boolean> = {};
  for (const p of [...modified, ...deleted]) {
    dirty_files[p] = true;
  }
  return { modified, deleted, dirty_files };
}

Deno.test("workingTreeBannerText counts modified and deleted files", () => {
  const given = status(["a.go", "b.go"], ["c.go"]);
  const actual = workingTreeBannerText(given);
  assertEquals(actual, "Uncommitted changes detected: 2 modified, 1 deleted.");
});

Deno.test("workingTreeBannerText reports zero counts when only one kind is present", () => {
  const given = status(["a.go"], []);
  const actual = workingTreeBannerText(given);
  assertEquals(actual, "Uncommitted changes detected: 1 modified, 0 deleted.");
});

Deno.test("workingTreeBannerText returns null for a clean tree", () => {
  const given = status([], []);
  const actual = workingTreeBannerText(given);
  assertEquals(actual, null);
});

Deno.test("isFileDirty reports a file in the dirty set", () => {
  const given = status(["a.go"], ["b.go"]);
  assertEquals(isFileDirty(given, "a.go"), true);
  assertEquals(isFileDirty(given, "b.go"), true);
});

Deno.test("isFileDirty reports false for a clean file", () => {
  const given = status(["a.go"], []);
  assertEquals(isFileDirty(given, "untouched.go"), false);
});

Deno.test("isFileDirty reports false when the status is null", () => {
  assertEquals(isFileDirty(null, "a.go"), false);
});
