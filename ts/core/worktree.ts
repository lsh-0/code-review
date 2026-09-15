// pure helpers over the working-tree status: deriving the page-wide banner
// text and the per-file dirty check. These hold no DOM or bridge dependency,
// so they are unit-testable in isolation; the render layer (`main.ts`) reads
// them to decide what to show.

import type { WorkingTreeStatus } from "./types.ts";

// the message for the page-wide uncommitted-changes banner, or null when the
// tree is clean and the banner should stay hidden.
//
// The message reports only that uncommitted changes exist, without counting
// them: which files are affected is per-file detail that the file list and the
// per-file banner already carry, and a bare number is not actionable on its own.
// Reporting a bare condition also means the banner cannot go stale while it
// holds, which is what lets the working-tree watcher suspend itself while the
// banner is up.
export function workingTreeBannerText(
  status: WorkingTreeStatus,
): string | null {
  if (status.modified.length === 0 && status.deleted.length === 0) {
    return null;
  }
  return "Uncommitted changes detected.";
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
