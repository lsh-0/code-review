// pure selection logic for the file list: given the current ordered selection
// and a clicked path, compute the next selection. A plain click replaces the
// whole selection with the one path; a ctrl/cmd (additive) click toggles the
// path in or out, but never empties the set — toggling out the last remaining
// file is a no-op, so a file is always shown while a file (not the overview) is
// active.

// the next selection after clicking `path`. `additive` is the ctrl/cmd modifier:
// false replaces the selection with `[path]`; true appends `path` when absent or
// removes it when present, except that removing the last file leaves the
// selection unchanged.
export function nextSelection(
  current: readonly string[],
  path: string,
  additive: boolean,
): string[] {
  if (!additive) {
    return [path];
  }
  if (current.includes(path)) {
    if (current.length === 1) {
      return [...current];
    }
    return current.filter((p) => p !== path);
  }
  return [...current, path];
}

// the primary (anchor) file of a selection: the last-toggled path, or the empty
// string when nothing is selected. The render layer keeps this as its single
// "current file" notion for the file-list highlight fallback and refresh paths.
export function primaryFile(selection: readonly string[]): string {
  return selection.length === 0 ? "" : selection[selection.length - 1];
}
