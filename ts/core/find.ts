// pure in-page-find logic: locating a search term's occurrences within a string
// and stepping a selection through a match list with wraparound. The render
// layer walks the diff's text nodes and paints highlights; these functions make
// every decision it needs without touching the DOM, so they are unit-testable
// without a browser. WebKitGTK does not wire up the browser's native Ctrl-f, so
// the find feature is built on top of these.

// a single match within a searched string: a half-open `[start, start+length)`
// range over the original (un-lowercased) string, so the render layer can slice
// the exact source text it highlights.
export interface FindMatch {
  start: number;
  length: number;
}

// the non-overlapping, case-insensitive occurrences of `needle` in `haystack`,
// in order. An empty or whitespace-only `needle` matches nothing (so clearing
// the find box removes every highlight). Search advances past each whole match,
// so overlapping occurrences (e.g. "aa" in "aaa") yield the leftmost set.
export function findMatches(haystack: string, needle: string): FindMatch[] {
  const term = needle.trim();
  if (term === "") {
    return [];
  }
  const hay = haystack.toLowerCase();
  const lower = term.toLowerCase();
  const matches: FindMatch[] = [];
  let from = 0;
  while (true) {
    const at = hay.indexOf(lower, from);
    if (at < 0) {
      break;
    }
    matches.push({ start: at, length: term.length });
    from = at + term.length;
  }
  return matches;
}

// the index of the next match after `current`, wrapping past the end back to the
// first. Returns 0 for an empty list (no selection to step). A `current` outside
// the list is treated as before the start, so the first step selects index 0.
export function nextMatchIndex(current: number, count: number): number {
  if (count <= 0) {
    return 0;
  }
  if (current < 0 || current >= count) {
    return 0;
  }
  return (current + 1) % count;
}

// the index of the previous match before `current`, wrapping past the start back
// to the last. Returns 0 for an empty list. A `current` outside the list is
// treated as after the end, so the first step selects the last index.
export function prevMatchIndex(current: number, count: number): number {
  if (count <= 0) {
    return 0;
  }
  if (current < 0 || current >= count) {
    return count - 1;
  }
  return (current - 1 + count) % count;
}
