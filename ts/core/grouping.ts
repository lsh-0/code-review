// pure grouping for the changed-file list. A grouper partitions the files into
// ordered, headed groups; "none" yields a single unheaded group so the render
// layer has one code path. Grouping is data-driven so new groupers (e.g. by
// file extension) are cheap to add: define one `Grouper` and list it.

import type { DiffFileMeta } from "./types.ts";

// a partition of the file list under a heading. `label` is empty for the
// single group "none" produces, signalling the render layer to omit the heading.
export interface FileGroup {
  id: string;
  label: string;
  files: DiffFileMeta[];
}

// the external state a grouper may read about each file. Kept explicit (rather
// than reaching into view state) so grouping stays pure and testable.
export interface GroupingContext {
  isMarked: (filePath: string) => boolean;
}

// a named way to partition the file list. `label` is the dropdown text; `group`
// returns the ordered groups, preserving each file's order within its group.
export interface Grouper {
  key: string;
  label: string;
  group: (files: DiffFileMeta[], ctx: GroupingContext) => FileGroup[];
}

// the single unheaded group: the flat list as it renders today.
const noneGrouper: Grouper = {
  key: "none",
  label: "None",
  group: (files) => [{ id: "all", label: "", files }],
};

// marked/unmarked, unmarked ("Needs review") first so files still needing
// attention rise to the top. Empty groups are dropped.
const markedGrouper: Grouper = {
  key: "marked",
  label: "Marked / unmarked",
  group: (files, ctx) => {
    const unmarked = files.filter((f) => !ctx.isMarked(f.Path));
    const marked = files.filter((f) => ctx.isMarked(f.Path));
    return [
      { id: "unmarked", label: "Unmarked", files: unmarked },
      { id: "marked", label: "Marked", files: marked },
    ].filter((g) => g.files.length > 0);
  },
};

// the groupers offered in the dropdown, in display order. "none" is the default.
export const groupers: Grouper[] = [noneGrouper, markedGrouper];

// the grouper for `key`, falling back to "none" for an unknown key.
export function grouperFor(key: string): Grouper {
  return groupers.find((g) => g.key === key) ?? noneGrouper;
}

// partition `files` with the grouper named `key` under context `ctx`.
export function groupFiles(
  files: DiffFileMeta[],
  key: string,
  ctx: GroupingContext,
): FileGroup[] {
  return grouperFor(key).group(files, ctx);
}
