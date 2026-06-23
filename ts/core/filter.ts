// pure filtering for the changed-file list: a case-insensitive substring match
// of the typed term against the whole file path. An empty (or whitespace-only)
// term matches everything, so clearing the box restores the full list.

// whether `filePath` matches the filter `term` (case-insensitive substring).
export function fileMatchesFilter(filePath: string, term: string): boolean {
  const needle = term.trim().toLowerCase();
  if (needle === "") {
    return true;
  }
  return filePath.toLowerCase().includes(needle);
}
