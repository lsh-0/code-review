// selection and region orchestration: which surface is shown (files or the
// overview), and the region re-renders that keep the DOM a projection of
// `state`. Any view change — selection, mutation, growth — funnels through
// here: regions re-render wholesale from state (pane scroll preserved, find
// highlights re-applied) rather than the DOM being patched in place.

import * as api from "../client.ts";
import { isFileDirty, workingTreeBannerText } from "../core/worktree.ts";
import { nextSelection } from "../core/selection.ts";
import { byId, clear } from "../dom.ts";
import { currentFile, state } from "./state.ts";
import {
  combinedFileHeader,
  renderCombinedDiff,
  renderFileHunks,
} from "./hunks.ts";
import { renderFileList } from "./filelist.ts";
import { renderOverview } from "./overview.ts";
import { reapplyFind } from "./find.ts";
import {
  ensureFileDiffs,
  loadCommentedFiles,
  loadComments,
  reloadReview,
} from "./load.ts";

// the file-list wiring: selection is this module's job, so the list's callbacks
// are built here once.
export const fileListCallbacks = {
  onSelectFile: (f: string, additive = false) => void selectFile(f, additive),
  onSelectOverview: () => void selectOverview(),
};

// select a file and render the diff pane from already-loaded state — no diff
// recompute, no git. A plain click (additive false) selects just `filePath`; a
// ctrl/cmd-click (additive true) toggles it in or out of the current selection.
// A plain re-click of a single already-selected file is a no-op so scroll
// survives (window state would survive regardless — it lives in state).
export async function selectFile(
  filePath: string,
  additive = false,
): Promise<void> {
  const alreadyShown = !state.overviewActive &&
    state.selectedFiles.length === 1 && state.selectedFiles[0] === filePath;
  if (!additive && alreadyShown) {
    return;
  }
  state.overviewActive = false;
  state.selectedFiles = nextSelection(state.selectedFiles, filePath, additive);
  await renderSelection();
}

// render the current file selection into the diff pane: load every selected
// file's hunks and comments, mark the file list, label the sticky header, and
// render the combined (or single-file) view. Resets scroll, since the selection
// has changed.
export async function renderSelection(): Promise<void> {
  const selected = state.selectedFiles;

  const diffView = byId("diff-view");
  if (diffView) {
    diffView.scrollTop = 0;
  }

  setActiveItem(selected);
  labelSelection(selected);
  renderFileDirtyBanner(currentFile());

  await ensureFileDiffs(selected);
  await Promise.all(selected.map((p) => loadComments(p)));
  renderCombinedDiff(selected);
  reapplyFind();
}

// re-render one file's region from state, preserving the pane's scroll
// position. The region is the file's `[data-file]` section in the combined
// view, or the whole content element in the single-file view. A file that is
// no longer shown (navigated away, overview active) has no region — the call
// is a no-op, which is what makes late async results harmless.
export function rerenderFileRegion(filePath: string): void {
  if (state.overviewActive || !state.selectedFiles.includes(filePath)) {
    return;
  }

  const pane = byId("diff-view");
  const scrollTop = pane?.scrollTop ?? 0;

  if (state.selectedFiles.length === 1) {
    const content = byId("diff-content");
    if (!content) {
      return;
    }
    clear(content);
    renderFileHunks(content, filePath);
  } else {
    const section = document.querySelector<HTMLElement>(
      `.diff-file-section[data-file="${cssEscape(filePath)}"]`,
    );
    if (!section) {
      return;
    }
    clear(section);
    section.appendChild(combinedFileHeader(filePath));
    renderFileHunks(section, filePath);
  }

  if (pane) {
    pane.scrollTop = scrollTop;
  }
  reapplyFind();
}

// show the review overview, reloading state first so it reflects an agent's
// edits.
export async function selectOverview(): Promise<void> {
  state.overviewActive = true;
  state.selectedFiles = [];

  const diffView = byId("diff-view");
  if (diffView) {
    diffView.scrollTop = 0;
  }
  setActiveOverview();
  const nameEl = byId("current-file-name");
  if (nameEl) {
    nameEl.textContent = "Review overview";
  }
  removeBrowseLink();
  renderFileDirtyBanner("");

  await reloadReview();
  const { reviewComments, files } = await loadCommentedFiles();
  await renderOverview(reviewComments, files);
  reapplyFind();
}

// rebuild the overview wholesale (used after a mutation while the overview is
// active, since it has no window state to preserve).
export function rebuildOverview(): void {
  void loadCommentedFiles().then(({ reviewComments, files }) => {
    void renderOverview(reviewComments, files).then(() => reapplyFind());
    renderFileList(fileListCallbacks);
  });
}

// escape a string for safe use in a CSS attribute selector. File paths contain
// `/` and `.`, which are selector-safe, but quotes or other metacharacters in a
// path would break the `[data-file="…"]` query; escaping guards against that.
function cssEscape(value: string): string {
  const cssApi = (globalThis as { CSS?: { escape?: (s: string) => string } })
    .CSS;
  if (cssApi?.escape) {
    return cssApi.escape(value);
  }
  // fallback for the test shim, which has no `CSS.escape`: escape the quote and
  // backslash that would otherwise terminate or corrupt the quoted selector.
  return value.replace(/["\\]/g, "\\$&");
}

// label the sticky pane header for the selection: a single file shows its path
// and a top browse button (the pre-multi-select look); several files show a
// count, with each section carrying its own filename header and browse button.
function labelSelection(selected: string[]): void {
  const nameEl = byId("current-file-name");
  if (selected.length === 1) {
    if (nameEl) {
      nameEl.textContent = selected[0];
    }
    renderBrowseLink(selected[0]);
    return;
  }
  if (nameEl) {
    nameEl.textContent = `${selected.length} files`;
  }
  removeBrowseLink();
}

// mark every file-list item in `selected` active, clearing the rest.
function setActiveItem(selected: string[]): void {
  const set = new Set(selected);
  for (const item of document.querySelectorAll<HTMLElement>(".file-item")) {
    const path = item.dataset.path;
    item.classList.toggle("active", path !== undefined && set.has(path));
  }
}

function setActiveOverview(): void {
  for (const item of document.querySelectorAll<HTMLElement>(".file-item")) {
    item.classList.toggle("active", item.classList.contains("overview-item"));
  }
}

// render (or replace) the "browse" link in the file header that opens the
// selected file externally.
function renderBrowseLink(filePath: string): void {
  const header = byId("current-file-header");
  if (!header) {
    return;
  }
  removeBrowseLink();
  const link = document.createElement("button");
  link.id = "browse-link";
  link.classList.add("browse-link");
  link.textContent = "browse";
  link.setAttribute("title", "Open this file in the preferred application");
  link.addEventListener("click", () => {
    void api.browseFile(filePath).catch(() =>
      globalThis.alert(`Could not open ${filePath}`)
    );
  });
  header.appendChild(link);
}

function removeBrowseLink(): void {
  byId("browse-link")?.remove();
}

// render the page-wide green banner from the cached status: shown when the tree
// is dirty, hidden when clean.
//
// The banner's visibility drives the working-tree watcher's suspension, since
// this banner is that watcher's alert. Raising it suspends the watcher, which
// then reports only the return to a clean tree; that return hides the banner
// here and resumes normal reporting. Both calls are idempotent, so re-rendering
// an unchanged banner costs nothing.
export function renderWorkingTreeBanner(): void {
  const banner = byId("working-tree-banner");
  const text = byId("working-tree-banner-text");
  if (!banner || !text || !state.workingTreeStatus) {
    return;
  }
  const message = workingTreeBannerText(state.workingTreeStatus);
  if (message === null) {
    banner.classList.add("hidden");
    void api.resumeWatch("worktree");
    return;
  }
  text.textContent = message;
  banner.classList.remove("hidden");
  void api.suspendWatch("worktree");
}

// render the per-file orange banner for the currently viewed file: shown when
// that file has uncommitted changes, hidden otherwise (and on the overview,
// where `filePath` is empty). The difftool button is wired once in main.ts;
// this only toggles visibility.
export function renderFileDirtyBanner(filePath: string): void {
  const banner = byId("file-dirty-banner");
  if (!banner) {
    return;
  }
  if (filePath !== "" && isFileDirty(state.workingTreeStatus, filePath)) {
    banner.classList.remove("hidden");
  } else {
    banner.classList.add("hidden");
  }
}
