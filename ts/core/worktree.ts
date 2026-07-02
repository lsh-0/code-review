// pure helpers over the working-tree status: deriving the page-wide banner
// text and the per-file dirty check. These hold no DOM or bridge dependency,
// so they are unit-testable in isolation; the render layer (`main.ts`) reads
// them to decide what to show.

import type { WorkingTreeStatus } from "./types.ts";

// the message for the page-wide uncommitted-changes banner, or null when the
// tree is clean and the banner should stay hidden. Counts tracked files
// modified and deleted (untracked files are not part of `status`), pluralising
// each label. A status with neither modified nor deleted files yields null.
export function workingTreeBannerText(
  status: WorkingTreeStatus,
): string | null {
  const modified = status.modified.length;
  const deleted = status.deleted.length;
  if (modified === 0 && deleted === 0) {
    return null;
  }
  return `Uncommitted changes detected: ${modified} modified, ${deleted} deleted.`;
}

// whether a given file path has uncommitted working-tree changes, per the
// status' `dirty_files` set. A path absent from the set (or a clean tree) is
// not dirty.
export function isFileDirty(
  status: WorkingTreeStatus | null,
  filePath: string,
): boolean {
  return status?.dirty_files[filePath] === true;
}
