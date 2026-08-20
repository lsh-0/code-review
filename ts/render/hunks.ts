// renders a file's diff as windows onto the file: each window is one block of
// the pure window plan — hunk segments (headers and diff lines) and context
// runs drawn from the fetched file body — with grow affordances at edges
// facing hidden lines. Everything rendered here derives from `state`; a
// re-render of the region from the same state reproduces the same view. The
// overview's comment-only branch renders from the hunks directly (windows are
// a file-page concern).

import { type DiffHunk, type DiffLine, LineContext } from "../core/types.ts";
import {
  defaultWindows,
  type GrowDirection,
  growDown,
  growStep,
  growUp,
  growWindow,
  hunkReachedEOF,
  hunkSpan,
  missingRanges,
  normaliseWindows,
  windowPlan,
  windowsContain,
  type WindowSegment,
} from "../core/windows.ts";
import { hunkHasComments, overviewVisibleLines } from "../core/overview.ts";
import { byId, clear, el } from "../dom.ts";
import { browseFile, getFileLines } from "../client.ts";
import { outdatedCommentsBlock, topLevelCommentsBlock } from "./comments.ts";
import {
  appendCommentThread,
  createDiffLine,
  hasCommentsForLine,
} from "./diff.ts";
import {
  commentsFor,
  diffFile,
  type FileViewState,
  placedCommentLines,
  state,
} from "./state.ts";
import { rerenderFileRegion } from "./view.ts";

// render a single file's diff into #diff-content. The single-file view renders
// its hunks directly into the content element (no section wrapper), so its DOM
// is identical to the pre-multi-select layout.
export function renderDiff(filePath: string): void {
  const content = byId("diff-content");
  if (!content) {
    return;
  }
  clear(content);
  renderFileHunks(content, filePath);
}

// render several files' diffs into #diff-content as one stacked view: one
// labelled section per path, in the given order, each a filename header followed
// by that file's full windows. A single path renders as the plain single-file
// view (no section chrome) so the common case stays unchanged. Each section
// carries a `data-file` attribute so per-file re-renders can scope to it.
export function renderCombinedDiff(paths: string[]): void {
  if (paths.length <= 1) {
    renderDiff(paths[0] ?? "");
    return;
  }
  const content = byId("diff-content");
  if (!content) {
    return;
  }
  clear(content);
  for (const filePath of paths) {
    const section = el("div", { classes: ["diff-file-section"] });
    section.dataset.file = filePath;
    section.appendChild(combinedFileHeader(filePath));
    renderFileHunks(section, filePath);
    content.appendChild(section);
  }
}

// a combined-view section header: the filename and a browse button to open the
// file externally. The filename is a plain label (the file is already shown, so
// it does not link anywhere); the browse button mirrors the single-file header's.
export function combinedFileHeader(filePath: string): HTMLElement {
  const header = el("div", { classes: ["diff-file-section-header"] });
  header.appendChild(
    el("span", { classes: ["diff-file-section-name"], text: filePath }),
  );
  header.appendChild(
    el("button", {
      classes: ["browse-link"],
      text: "browse",
      attrs: { title: "Open this file in the preferred application" },
      onClick: () => {
        void browseFile(filePath).catch(() =>
          globalThis.alert(`Could not open ${filePath}`)
        );
      },
    }),
  );
  return header;
}

// the window state for a file, created on first render: default windows from
// the hunks and the placed comment lines. Kept across file switches; a diff
// refresh clears the map, so the next render recomputes (collapses to)
// defaults from the refreshed data.
function ensureFileView(filePath: string, hunks: DiffHunk[]): FileViewState {
  let view = state.fileViews.get(filePath);
  if (!view) {
    view = {
      windows: defaultWindows(hunks, placedCommentLines(filePath), 0),
      fileLines: new Map(),
      totalLines: 0,
      fetchFailed: false,
      fetching: false,
    };
    state.fileViews.set(filePath, view);
  }
  return view;
}

// make sure every placed comment line has a window: mutations (or an agent's
// edits arriving via reload) can place a comment outside the current windows,
// and it must not be invisible. Grown windows are untouched — this only adds
// islands, mirroring the default-window rule.
export function ensureCommentWindows(filePath: string): void {
  const file = diffFile(filePath);
  const view = state.fileViews.get(filePath);
  if (!file || !view) {
    return;
  }
  const missing = placedCommentLines(filePath)
    .filter((line) => !windowsContain(view.windows, line));
  if (missing.length === 0) {
    return;
  }
  const islands = defaultWindows([], missing, view.totalLines);
  view.windows = normaliseWindows(
    [...view.windows, ...islands],
    view.totalLines,
  );
}

// render a file's diff into `parent`. The single-file view (`overviewOnly`
// false) renders the file's windows with grow affordances. The overview
// (`overviewOnly` true) renders read-only and only the hunks carrying a
// comment, each trimmed to the context window around its commented lines.
// Binary files render a placeholder; a missing entry renders nothing.
export function renderFileHunks(
  parent: Element,
  filePath: string,
  overviewOnly = false,
): void {
  const file = diffFile(filePath);
  if (!file) {
    return;
  }

  // top-level (anchorless) file comments render at the very top of the file, for
  // text and binary files alike — a binary file has no line to comment on, so
  // this is its only comment surface. The "Add comment" button shows only in the
  // single-file view; the overview renders these read-only (no button, and only
  // when comments exist).
  const topLevel = topLevelCommentsBlock(
    filePath,
    commentsFor(filePath),
    !overviewOnly,
  );
  if (topLevel) {
    parent.appendChild(topLevel);
  }

  if (file.Binary) {
    parent.appendChild(
      el("div", {
        classes: ["binary-placeholder"],
        text: "binary file, cannot diff",
      }),
    );
    return;
  }

  // a pure rename (git detected the rename and the file has no content change)
  // would otherwise render a blank pane. A rename with edits keeps its hunks and
  // falls through to the normal rendering below.
  if (file.Renamed && file.Hunks.length === 0) {
    parent.appendChild(
      el("div", {
        classes: ["rename-placeholder"],
        text: file.OldPath
          ? `renamed from ${file.OldPath}, no changes`
          : "renamed, no changes",
      }),
    );
    return;
  }

  if (overviewOnly) {
    renderOverviewHunks(parent, filePath, file.Hunks);
    return;
  }

  const view = ensureFileView(filePath, file.Hunks);
  ensureCommentWindows(filePath);

  // untethered comments: outdated ones, plus live anchored comments whose line
  // no window can show (beyond the end of the file once the total is known).
  // The latter keep their anchors; this is a render fallback only.
  const unplaceable = commentsFor(filePath).filter((c) =>
    !c.parent_id && !c.outdated && c.line > 0 &&
    !windowsContain(view.windows, c.line)
  );
  const outdated = outdatedCommentsBlock(
    filePath,
    commentsFor(filePath),
    unplaceable,
  );
  if (outdated) {
    parent.appendChild(outdated);
  }

  for (let i = 0; i < view.windows.length; i++) {
    parent.appendChild(renderWindow(filePath, file, view, i));
  }

  // fetch any context lines the windows need but the cache lacks, then
  // re-render this region once. The re-rendered pass finds nothing missing, so
  // this cannot loop; a failed fetch flags the file and stops retrying.
  void fetchMissingLines(filePath).then((fetched) => {
    if (fetched) {
      rerenderFileRegion(filePath);
    }
  });
}

// render one window: a grow-up affordance at its top edge, its plan's segments
// (hunk headers and lines, context runs from the fetched body), and a
// grow-down affordance at its bottom edge. Affordances facing an exhausted
// file boundary render disabled-but-present, so navigation does not shift.
function renderWindow(
  filePath: string,
  file: { Hunks: DiffHunk[] },
  view: FileViewState,
  index: number,
): HTMLElement {
  const window = view.windows[index];
  const container = el("div", { classes: ["diff-hunk"] });

  container.appendChild(
    growRow(filePath, view, index, growUp, growUpDisabled(view, index)),
  );

  const segments = windowPlan(window, file.Hunks);
  let firstHunkInWindow = true;
  for (const segment of segments) {
    if (segment.kind === "hunk") {
      const hunk = file.Hunks[segment.hunkIndex];
      const header = el("div", {
        classes: ["hunk-header"],
        text:
          `@@ -${hunk.OldStart},${hunk.OldLines} +${hunk.NewStart},${hunk.NewLines} @@`,
      });
      // hunks after the first in a merged window read as one continuous block:
      // their headers stay in the DOM (navigation, tests) but hidden.
      if (!firstHunkInWindow) {
        header.classList.add("joined-hidden");
      }
      firstHunkInWindow = false;
      container.appendChild(header);
      for (const line of hunk.Lines) {
        const lineElem = createDiffLine(line, filePath);
        container.appendChild(lineElem);
        appendCommentThread(container, lineElem, filePath, line.NewLineNo);
      }
      continue;
    }
    appendContextRun(container, filePath, view, segment);
  }

  container.appendChild(
    growRow(
      filePath,
      view,
      index,
      growDown,
      growDownDisabled(file, view, index),
    ),
  );
  return container;
}

// append a context run's lines from the fetched body. A line not yet fetched
// renders nothing — the fetch pass that created the need re-renders the region
// when the lines arrive.
function appendContextRun(
  container: HTMLElement,
  filePath: string,
  view: FileViewState,
  segment: Extract<WindowSegment, { kind: "context" }>,
): void {
  for (let n = segment.start; n <= segment.end; n++) {
    const content = view.fileLines.get(n);
    if (content === undefined) {
      continue;
    }
    const line: DiffLine = {
      Type: LineContext,
      Content: content,
      OldLineNo: n + segment.oldOffset,
      NewLineNo: n,
    };
    const lineElem = createDiffLine(line, filePath);
    container.appendChild(lineElem);
    appendCommentThread(container, lineElem, filePath, n);
  }
}

// whether the grow-up affordance of the window at `index` has nothing to
// reveal: only the first window can exhaust upward, at line 1. Between-window
// affordances always face a gap of at least the merge threshold.
function growUpDisabled(view: FileViewState, index: number): boolean {
  return index === 0 && view.windows[0].start <= 1;
}

// whether the grow-down affordance has nothing to reveal. Only the last window
// can exhaust downward: at the known end of the file, or — before any fetch
// has reported a total — when the window ends exactly at a last hunk that
// visibly reached end-of-file.
function growDownDisabled(
  file: { Hunks: DiffHunk[] },
  view: FileViewState,
  index: number,
): boolean {
  if (index !== view.windows.length - 1) {
    return false;
  }
  const window = view.windows[index];
  if (view.totalLines > 0) {
    return window.end >= view.totalLines;
  }
  const lastHunk = file.Hunks[file.Hunks.length - 1];
  if (!lastHunk) {
    return false;
  }
  return window.end === hunkSpan(lastHunk).end &&
    hunkReachedEOF(lastHunk.Lines);
}

// a grow affordance row. Clicking grows the window in `direction` by the step,
// fetches whatever the grown window needs, and re-renders the region — the
// merge/exhaustion outcomes all fall out of the window normalisation.
function growRow(
  filePath: string,
  view: FileViewState,
  index: number,
  direction: GrowDirection,
  disabled: boolean,
): HTMLElement {
  const arrow = direction === growDown ? "↓" : "↑";
  const row = el("div", {
    classes: ["expand-row"],
    text: `${arrow} expand ${growStep} lines`,
  });
  if (disabled) {
    row.classList.add("disabled");
  }
  row.addEventListener("click", () => {
    if (row.classList.contains("disabled")) {
      return;
    }
    view.windows = growWindow(
      view.windows,
      index,
      direction,
      growStep,
      view.totalLines,
    );
    void fetchMissingLines(filePath).then(() => rerenderFileRegion(filePath));
  });
  return row;
}

// fetch every context range the file's windows need but the cache lacks, into
// the view state. Reports whether anything was fetched. The first result also
// teaches the view the file's total line count, after which the windows are
// re-normalised (clamping any range that ran past the end). A failed fetch
// (binary, vanished path) flags the file so it is not re-requested.
async function fetchMissingLines(filePath: string): Promise<boolean> {
  const file = diffFile(filePath);
  const view = state.fileViews.get(filePath);
  if (!file || !view || view.fetchFailed || view.fetching) {
    return false;
  }

  const ranges = view.windows.flatMap((w) =>
    missingRanges(
      windowPlan(w, file.Hunks),
      (n) => view.fileLines.has(n),
    )
  );
  if (ranges.length === 0) {
    return false;
  }

  view.fetching = true;
  let fetched = false;
  try {
    for (const range of ranges) {
      const result = await getFileLines(
        filePath,
        range.start,
        range.end,
        range.oldOffset,
      );
      for (const line of result.Lines) {
        view.fileLines.set(line.NewLineNo, line.Content);
      }
      view.totalLines = result.TotalNew;
      fetched = true;
    }
  } catch {
    view.fetchFailed = true;
  } finally {
    view.fetching = false;
  }

  view.windows = normaliseWindows(view.windows, view.totalLines);
  return fetched;
}

// the overview's read-only branch: only the hunks carrying a comment, each
// trimmed to the context window around its commented lines.
function renderOverviewHunks(
  parent: Element,
  filePath: string,
  hunks: DiffHunk[],
): void {
  for (const hunk of hunks) {
    if (!hunkHasComments(hunk, (n) => hasCommentsForLine(filePath, n))) {
      continue;
    }
    const hunkElem = el("div", { classes: ["diff-hunk"] });
    hunkElem.appendChild(
      el("div", {
        classes: ["hunk-header"],
        text:
          `@@ -${hunk.OldStart},${hunk.OldLines} +${hunk.NewStart},${hunk.NewLines} @@`,
      }),
    );
    appendCommentedHunkLines(hunkElem, filePath, hunk);
    parent.appendChild(hunkElem);
  }
}

// append a commented hunk's lines to `parent`, but only those within the
// context window of a commented line (via the pure overviewVisibleLines), with
// each comment thread embedded after its line. Lines outside every window are
// dropped, so a comment in a large hunk shows just its neighbourhood.
function appendCommentedHunkLines(
  parent: HTMLElement,
  filePath: string,
  hunk: DiffHunk,
): void {
  const visible = overviewVisibleLines(
    hunk,
    (n) => hasCommentsForLine(filePath, n),
  );
  for (let i = 0; i < hunk.Lines.length; i++) {
    if (!visible[i]) {
      continue;
    }
    const line = hunk.Lines[i];
    const lineElem = createDiffLine(line, filePath);
    parent.appendChild(lineElem);
    appendCommentThread(parent, lineElem, filePath, line.NewLineNo);
  }
}
