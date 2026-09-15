// the frontend entry point: assembles the render modules, wires selection,
// refresh, the comment-mutation callbacks, the modals, the external-change
// banner, and the zoom/keyboard/wheel handlers, then loads the initial view.
// Bundled to `assets/review.js` and loaded by `index.html`.

import * as api from "./client.ts";
import type { WorkingTreeStatus } from "./core/types.ts";
import { isFileDirty, workingTreeBannerText } from "./core/worktree.ts";
import { byId, clear, el, requireId } from "./dom.ts";
import { state } from "./render/state.ts";
import { clampPaneWidth } from "./core/panes.ts";
import { type CommentActions } from "./render/comments.ts";
import { sourceToggles } from "./core/comments.ts";
import { type DiffCallbacks, renderCombinedDiff } from "./render/hunks.ts";
import { renderFileList } from "./render/filelist.ts";
import { groupers } from "./core/grouping.ts";
import { renderOverview } from "./render/overview.ts";
import { type MutationContext } from "./render/mutate.ts";
import {
  deleteComment,
  hideCommentModal,
  hideEditCommentModal,
  hideReplyModal,
  ignoreComment,
  initModals,
  reactivateComment,
  resolveComment,
  saveComment,
  saveReply,
  showCommentModal,
  showEditCommentModal,
  showReplyModal,
  showReviewCommentModal,
  updateComment,
} from "./render/modals.ts";
import {
  ensureFileDiffs,
  loadAllComments,
  loadCommentedFiles,
  loadComments,
  loadDiffFiles,
  recomputeDiff,
  reloadReview,
} from "./render/load.ts";
import { nextSelection } from "./core/selection.ts";

const zoomMin = 0.5;
const zoomMax = 3.0;
const zoomStep = 0.1;

// the most recently fetched working-tree status, cached so the per-file dirty
// check on file selection reads it without a fresh git call. Refreshed on load
// and on every full refresh; null until the first fetch succeeds.
let workingTreeStatus: WorkingTreeStatus | null = null;

// the comment actions, wired once: each opens a modal or calls a mutation
// handler. Threaded into every comment thread the render layer builds.
const actions: CommentActions = {
  onReply: (f, id) => showReplyModal(f, id),
  onEdit: (f, id, content) => showEditCommentModal(f, id, content),
  onResolve: (f, id) => void resolveComment(f, id),
  onIgnore: (f, id) => void ignoreComment(f, id),
  onReactivate: (f, id) => void reactivateComment(f, id),
  onDelete: (f, id) => void deleteComment(f, id),
};

const diffCallbacks: DiffCallbacks = {
  actions,
  onAddComment: (f, lineNo) => showCommentModal(f, lineNo),
};

// rebuild the overview wholesale (used after a mutation while the overview is
// active, since it has no diff state to preserve).
function rebuildOverview(): void {
  void loadCommentedFiles().then(({ reviewComments, files }) => {
    void renderOverview(reviewComments, files, overviewCallbacks);
    renderFileList(fileListCallbacks);
  });
}

const mutationCtx: MutationContext = { actions, rebuildOverview };

const fileListCallbacks = {
  onSelectFile: (f: string, additive = false) => void selectFile(f, additive),
  onSelectOverview: () => void selectOverview(),
};

const overviewCallbacks = {
  diff: diffCallbacks,
  actions,
  // selecting a file from the overview always opens just that file.
  onSelectFile: (f: string) => void selectFile(f),
  onAddReviewComment: () => showReviewCommentModal(),
};

// select a file and render the diff pane from already-loaded state — no diff
// recompute, no git. A plain click (additive false) selects just `filePath`; a
// ctrl/cmd-click (additive true) toggles it in or out of the current selection.
// A plain re-click of a single already-selected file is a no-op so scroll and
// expanded context survive.
async function selectFile(filePath: string, additive = false): Promise<void> {
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
async function renderSelection(): Promise<void> {
  const selected = state.selectedFiles;

  const diffView = byId("diff-view");
  if (diffView) {
    diffView.scrollTop = 0;
  }

  setActiveItem(selected);
  labelSelection(selected);
  renderFileDirtyBanner(state.currentFile);

  await ensureFileDiffs(selected);
  await Promise.all(selected.map((p) => loadComments(p)));
  renderCombinedDiff(selected, diffCallbacks);
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

// show the review overview, reloading state first so it reflects an agent's
// edits.
async function selectOverview(): Promise<void> {
  state.overviewActive = true;
  state.currentFile = "";

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
  await renderOverview(reviewComments, files, overviewCallbacks);
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

// recompute the diff and reload state, then re-render the file list and the
// visible surface.
async function performFullRefresh(): Promise<void> {
  await recomputeDiff();
  await loadDiffFiles();
  await refreshWorkingTreeStatus();
  // an import by the CLI or an agent adds a source while the GUI is open, so the
  // toggles are rebuilt here rather than only at startup.
  await loadReviewInfo();
  renderSourceToggles();
  renderFileList(fileListCallbacks);
  if (state.overviewActive) {
    const { reviewComments, files } = await loadCommentedFiles();
    await renderOverview(reviewComments, files, overviewCallbacks);
    return;
  }

  // drop any selected file that the recompute removed from the diff, so a stale
  // path neither holds an active mark nor renders an empty section. The reload
  // also dropped the surviving files' hunks; `renderSelection` re-fetches them.
  const present = new Set(state.diffFiles.map((f) => f.Path));
  state.selectedFiles = state.selectedFiles.filter((p) => present.has(p));
  if (state.selectedFiles.length > 0) {
    await renderSelection();
  }
}

// refresh, flashing the Refresh button on completion.
async function triggerRefresh(): Promise<void> {
  await performFullRefresh();
  flashSuccess(byId("refresh-btn"));
}

// briefly flash a success colour on an element without changing its size/text.
function flashSuccess(elem: HTMLElement | null): void {
  if (!elem) {
    return;
  }
  elem.classList.add("flash-success");
  globalThis.setTimeout(() => elem.classList.remove("flash-success"), 400);
}

// copy the AI prompt to the clipboard, flashing the button on success.
async function copyStatePrompt(): Promise<void> {
  const prompt = await api.getStatePrompt();
  if (prompt === "") {
    return;
  }
  await navigator.clipboard.writeText(prompt);
  flashSuccess(byId("copy-prompt-btn"));
}

function applyZoom(): void {
  state.zoomLevel = Math.min(zoomMax, Math.max(zoomMin, state.zoomLevel));
  document.documentElement.style.setProperty("--zoom", String(state.zoomLevel));
}

// fetch the working-tree status, cache it, and render the page-wide banner.
// Called on load and after every full refresh, since a refresh may follow a
// new commit that changed what is dirty. A fetch failure leaves the previous
// state and banner untouched rather than clearing a still-valid warning.
async function refreshWorkingTreeStatus(): Promise<void> {
  try {
    workingTreeStatus = await api.getWorkingTreeStatus();
  } catch {
    return;
  }
  renderWorkingTreeBanner();
  renderFileDirtyBanner(state.currentFile);
}

// render the page-wide green banner from the cached status: shown with a count
// of modified and deleted tracked files when the tree is dirty, hidden when
// clean.
function renderWorkingTreeBanner(): void {
  const banner = byId("working-tree-banner");
  const text = byId("working-tree-banner-text");
  if (!banner || !text || !workingTreeStatus) {
    return;
  }
  const message = workingTreeBannerText(workingTreeStatus);
  if (message === null) {
    banner.classList.add("hidden");
    return;
  }
  text.textContent = message;
  banner.classList.remove("hidden");
}

// render the per-file orange banner for the currently viewed file: shown when
// that file has uncommitted changes, hidden otherwise (and on the overview,
// where `filePath` is empty). The difftool button is wired once in
// `setupEventHandlers`; this only toggles visibility and points it at the file.
function renderFileDirtyBanner(filePath: string): void {
  const banner = byId("file-dirty-banner");
  if (!banner) {
    return;
  }
  if (filePath !== "" && isFileDirty(workingTreeStatus, filePath)) {
    banner.classList.remove("hidden");
  } else {
    banner.classList.add("hidden");
  }
}

function showReviewChangedBanner(): void {
  byId("review-changed-banner")?.classList.remove("hidden");
}

function hideReviewChangedBanner(): void {
  byId("review-changed-banner")?.classList.add("hidden");
}

// wire the external-change banner: the backend's `review:changed` event raises
// it; its buttons refresh or dismiss.
function setupReviewChangedBanner(): void {
  const runtime =
    (globalThis as { runtime?: { EventsOn(ev: string, cb: () => void): void } })
      .runtime;
  runtime?.EventsOn("review:changed", () => showReviewChangedBanner());

  byId("banner-refresh-btn")?.addEventListener("click", () => {
    hideReviewChangedBanner();
    void triggerRefresh();
  });
  byId("banner-dismiss-btn")?.addEventListener(
    "click",
    () => hideReviewChangedBanner(),
  );
}

// wire the live working-tree updates: the backend's `worktree:changed` event
// fires when a tracked file changes on disk (or reverts), re-fetching the
// status and re-rendering both banners without a manual refresh.
function setupWorkingTreeWatcher(): void {
  const runtime =
    (globalThis as { runtime?: { EventsOn(ev: string, cb: () => void): void } })
      .runtime;
  runtime?.EventsOn("worktree:changed", () => void refreshWorkingTreeStatus());
}

// the keyboard nudge applied to the column width per arrow-key press.
const paneKeyboardStep = 24;

// wire the divider between the file list and the diff: dragging it (pointer or
// arrow keys) resizes the file-list column, clamped so a sliver of each pane
// always shows. In-session only — the width resets to the CSS default on reload.
function setupPaneResizer(): void {
  const resizer = byId("pane-resizer");
  const fileList = byId("file-list");
  if (!resizer || !fileList) {
    return;
  }

  const applyWidth = (widthPx: number): void => {
    fileList.style.width = `${
      clampPaneWidth(widthPx, globalThis.innerWidth)
    }px`;
  };

  resizer.addEventListener("pointerdown", (ev) => {
    ev.preventDefault();
    resizer.classList.add("dragging");
    resizer.setPointerCapture(ev.pointerId);

    const onMove = (move: PointerEvent): void => {
      // the column starts at the left edge, so its width is the cursor's x.
      applyWidth(move.clientX - fileList.getBoundingClientRect().left);
    };
    const onUp = (): void => {
      resizer.classList.remove("dragging");
      resizer.removeEventListener("pointermove", onMove);
    };
    resizer.addEventListener("pointermove", onMove);
    resizer.addEventListener("pointerup", onUp, { once: true });
    resizer.addEventListener("pointercancel", onUp, { once: true });
  });

  resizer.addEventListener("keydown", (ev) => {
    const delta = ev.key === "ArrowLeft"
      ? -paneKeyboardStep
      : ev.key === "ArrowRight"
      ? paneKeyboardStep
      : 0;
    if (delta === 0) {
      return;
    }
    ev.preventDefault();
    applyWidth(fileList.getBoundingClientRect().width + delta);
  });
}

// wire the "Group by" dropdown: populate it from the available groupers, select
// the current one, and re-render the list when the choice changes. In-session
// only — the grouping resets to the default ("none") on reload.
function setupGroupControl(): void {
  const select = byId("file-grouping");
  if (!(select instanceof HTMLSelectElement)) {
    return;
  }
  for (const grouper of groupers) {
    select.appendChild(
      el("option", { text: grouper.label, attrs: { value: grouper.key } }),
    );
  }
  select.value = state.fileGrouping;
  select.addEventListener("change", () => {
    state.fileGrouping = select.value;
    renderFileList(fileListCallbacks);
  });
}

// wire the file-list filter box: each keystroke updates the filter term and
// re-renders the list, hiding files whose path does not match. The current
// selection is left untouched, so an open diff stays open even when filtered out.
function setupFileFilter(): void {
  const input = byId("file-filter");
  if (!(input instanceof HTMLInputElement)) {
    return;
  }
  input.addEventListener("input", () => {
    state.fileFilter = input.value;
    renderFileList(fileListCallbacks);
  });
}

// dismiss a modal when the click both starts and ends on the backdrop itself.
function setupModalBackdrop(modal: HTMLElement): void {
  let mousedownTarget: EventTarget | null = null;
  modal.addEventListener("mousedown", (ev) => {
    mousedownTarget = ev.target;
  });
  modal.addEventListener("mouseup", (ev) => {
    if (mousedownTarget === modal && ev.target === modal) {
      modal.classList.remove("active");
    }
    mousedownTarget = null;
  });
}

// wire the toolbar, modal, zoom, and wheel handlers.
function setupEventHandlers(): void {
  document.addEventListener("keydown", (ev) => {
    const key = ev.key;
    if (key === "Escape" || key === "Esc") {
      if (byId("comment-modal")?.classList.contains("active")) {
        hideCommentModal();
      }
      if (byId("edit-comment-modal")?.classList.contains("active")) {
        hideEditCommentModal();
      }
      if (byId("reply-comment-modal")?.classList.contains("active")) {
        hideReplyModal();
      }
    }
    if (ev.ctrlKey) {
      switch (key) {
        case "=":
        case "+":
          ev.preventDefault();
          state.zoomLevel += zoomStep;
          applyZoom();
          break;
        case "-":
        case "_":
          ev.preventDefault();
          state.zoomLevel -= zoomStep;
          applyZoom();
          break;
        case "0":
          ev.preventDefault();
          state.zoomLevel = 1.0;
          applyZoom();
          break;
      }
    }
  });

  setupPaneResizer();
  setupGroupControl();
  setupFileFilter();

  byId("refresh-btn")?.addEventListener("click", () => void triggerRefresh());
  byId("copy-prompt-btn")?.addEventListener(
    "click",
    () => void copyStatePrompt(),
  );
  byId("import-btn")?.addEventListener("click", () => void triggerImport());
  byId("import-banner-dismiss")?.addEventListener("click", () => {
    byId("import-banner")?.classList.add("hidden");
  });
  byId("file-dirty-difftool-btn")?.addEventListener("click", () => {
    const filePath = state.currentFile;
    if (filePath === "") {
      return;
    }
    void api.openDiffToolForFile(filePath).catch(() =>
      globalThis.alert(`Could not open the diff tool for ${filePath}`)
    );
  });
  byId("save-comment-btn")?.addEventListener("click", () => void saveComment());
  byId("cancel-comment-btn")?.addEventListener(
    "click",
    () => hideCommentModal(),
  );
  byId("update-comment-btn")?.addEventListener(
    "click",
    () => void updateComment(),
  );
  byId("cancel-edit-comment-btn")?.addEventListener(
    "click",
    () => hideEditCommentModal(),
  );
  byId("save-reply-btn")?.addEventListener("click", () => void saveReply());
  byId("cancel-reply-btn")?.addEventListener("click", () => hideReplyModal());

  for (
    const id of ["comment-modal", "edit-comment-modal", "reply-comment-modal"]
  ) {
    const modal = byId(id);
    if (modal) {
      setupModalBackdrop(modal);
    }
  }

  // Ctrl+wheel zooms; otherwise scroll the nearest scrollable ancestor. The
  // native WebKit zoom is unreliable under Wails, so it is handled here.
  document.addEventListener("wheel", (ev) => {
    const deltaY = ev.deltaY;
    if (ev.ctrlKey) {
      ev.preventDefault();
      state.zoomLevel += deltaY < 0 ? zoomStep : -zoomStep;
      applyZoom();
      return;
    }

    let current: Element | null = ev.target as Element | null;
    while (current) {
      const overflowY = globalThis.getComputedStyle(current).getPropertyValue(
        "overflow-y",
      );
      if (overflowY === "auto" || overflowY === "scroll") {
        if (current.scrollHeight > current.clientHeight) {
          ev.preventDefault();
          current.scrollBy(0, deltaY);
          return;
        }
      }
      current = current.parentElement;
    }
  }, { passive: false });
}

// load the review info into the header, and the imported sources into the state
// the visibility toggles are built from.
async function loadReviewInfo(): Promise<void> {
  const info = await api.getReviewInfo();
  state.currentUser = info.current_user;
  state.sources = info.sources ?? [];
  const branchInfo = byId("branch-info");
  if (branchInfo) {
    branchInfo.textContent = `${info.source_branch} → ${info.target_branch}`;
  }
}

// render the source-visibility control from `sourceToggles`, which decides the
// entries and their order. Every source starts visible; the setting is in-session,
// like the grouping, and resets on reload.
//
// An empty entry list means there is no choice to offer (nothing imported), so the
// control is hidden rather than rendered empty.
export function renderSourceToggles(): void {
  const container = byId("source-toggles");
  if (!container) {
    return;
  }
  clear(container);

  const entries = sourceToggles(state.sources, state.hiddenSources);
  if (entries.length === 0) {
    container.classList.add("hidden");
    return;
  }
  container.classList.remove("hidden");

  for (const entry of entries) {
    const item = el("label", { classes: ["source-toggle"] });
    const box = el("input");
    box.setAttribute("type", "checkbox");
    (box as HTMLInputElement).checked = entry.visible;
    box.addEventListener("change", () => {
      if ((box as HTMLInputElement).checked) {
        state.hiddenSources.delete(entry.id);
      } else {
        state.hiddenSources.add(entry.id);
      }
      void reRenderForVisibility();
    });
    item.appendChild(box);
    item.appendChild(el("span", { text: entry.label }));
    container.appendChild(item);
  }
}

// re-render the file list and the visible surface after a visibility change. No
// bridge call is made: visibility is view state and the comments are already
// cached, so nothing is fetched and nothing is persisted.
async function reRenderForVisibility(): Promise<void> {
  renderFileList(fileListCallbacks);
  if (state.overviewActive) {
    const { reviewComments, files } = await loadCommentedFiles();
    await renderOverview(reviewComments, files, overviewCallbacks);
    return;
  }
  if (state.selectedFiles.length > 0) {
    await renderSelection();
  }
}

// prompt for an external review document, import it, and show what it did.
//
// The file dialog and the import both run in the backend. A dismissed dialog
// changes nothing and reports nothing. After a successful import the review info
// and every comment surface are reloaded, so the imported feedback and its new
// source toggle appear without a manual refresh.
async function triggerImport(): Promise<void> {
  const result = await api.importForeignReview();
  if (result.cancelled) {
    return;
  }

  await loadReviewInfo();
  await loadAllComments();
  renderSourceToggles();
  await reRenderForVisibility();

  flashSuccess(byId("import-btn"));
  showImportReport(result.lines);
}

// show an import's report in its own banner, dismissed by the reviewer.
//
// It has a banner of its own rather than borrowing the working-tree one, which the
// working-tree watcher owns and rewrites on every poll.
function showImportReport(lines: string[]): void {
  if (lines.length === 0) {
    return;
  }
  const banner = byId("import-banner");
  const text = byId("import-banner-text");
  if (!banner || !text) {
    return;
  }
  text.textContent = lines.join(" · ");
  banner.classList.remove("hidden");
}

async function initialize(): Promise<void> {
  // the modals and mutation patching share one context.
  initModals(mutationCtx);
  // ensure the structural elements exist before wiring (throws early if not).
  requireId("diff-content");

  // the Wails runtime injects `window.go` asynchronously and may not be present
  // at the page's load event; wait for it before the first bridge call rather
  // than throwing on a not-ready bridge (which would abort the whole load and
  // leave a blank shell).
  const ready = await api.waitForBridge();
  if (!ready) {
    showInitError("Backend bridge did not become ready.");
    return;
  }

  await loadReviewInfo();
  await loadDiffFiles();
  await refreshWorkingTreeStatus();
  renderSourceToggles();
  renderFileList(fileListCallbacks);
  setupEventHandlers();
  setupReviewChangedBanner();
  setupWorkingTreeWatcher();
}

// surface an initialisation failure on the page rather than failing to a blank
// shell, so a problem here is visible without opening dev tools. The bridge
// diagnostic is logged to the console (not shown on screen) to aid debugging a
// bridge that never became ready.
function showInitError(detail: string): void {
  const content = byId("diff-content");
  if (content) {
    content.textContent = `Failed to load the review: ${detail}`;
  }
  console.error("initialise failed:", detail, "|", api.bridgeDiagnostic());
}

globalThis.addEventListener("load", () => {
  initialize().catch((err) => showInitError(String(err)));
});
