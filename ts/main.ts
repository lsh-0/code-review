// the frontend entry point: assembles the render modules, wires selection,
// refresh, the comment-mutation callbacks, the modals, the external-change
// banner, and the zoom/keyboard/wheel handlers, then loads the initial view.
// Bundled to `assets/review.js` and loaded by `index.html`.

import * as api from "./client.ts";
import { byId, el, requireId } from "./dom.ts";
import { state } from "./render/state.ts";
import { clampPaneWidth } from "./core/panes.ts";
import { type CommentActions } from "./render/comments.ts";
import { type DiffCallbacks, renderDiff } from "./render/hunks.ts";
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
  ensureFileDiff,
  loadCommentedFiles,
  loadComments,
  loadDiffFiles,
  recomputeDiff,
  reloadReview,
} from "./render/load.ts";

const zoomMin = 0.5;
const zoomMax = 3.0;
const zoomStep = 0.1;

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
  onSelectFile: (f: string) => void selectFile(f),
  onSelectOverview: () => void selectOverview(),
};

const overviewCallbacks = {
  diff: diffCallbacks,
  actions,
  onSelectFile: (f: string) => void selectFile(f),
  onAddReviewComment: () => showReviewCommentModal(),
};

// select a file and render its diff from already-loaded state — no diff
// recompute, no git. Re-selecting the current file is a no-op so scroll and
// expanded context survive.
async function selectFile(filePath: string): Promise<void> {
  if (filePath === state.currentFile && !state.overviewActive) {
    return;
  }
  state.overviewActive = false;
  state.currentFile = filePath;

  const diffView = byId("diff-view");
  if (diffView) {
    diffView.scrollTop = 0;
  }

  setActiveItem(filePath);
  const nameEl = byId("current-file-name");
  if (nameEl) {
    nameEl.textContent = filePath;
  }
  renderBrowseLink(filePath);

  await ensureFileDiff(filePath);
  await loadComments(filePath);
  renderDiff(filePath, diffCallbacks);
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

  await reloadReview();
  const { reviewComments, files } = await loadCommentedFiles();
  await renderOverview(reviewComments, files, overviewCallbacks);
}

// mark the file-list item for `filePath` active, clearing the rest.
function setActiveItem(filePath: string): void {
  for (const item of document.querySelectorAll<HTMLElement>(".file-item")) {
    item.classList.toggle("active", item.dataset.path === filePath);
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
  renderFileList(fileListCallbacks);
  if (state.overviewActive) {
    const { reviewComments, files } = await loadCommentedFiles();
    await renderOverview(reviewComments, files, overviewCallbacks);
  } else if (state.currentFile !== "") {
    // the reload dropped this file's hunks; re-fetch before rendering.
    await ensureFileDiff(state.currentFile);
    renderDiff(state.currentFile, diffCallbacks);
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

// load the review info into the header.
async function loadReviewInfo(): Promise<void> {
  const info = await api.getReviewInfo();
  state.currentUser = info.current_user;
  const branchInfo = byId("branch-info");
  if (branchInfo) {
    branchInfo.textContent = `${info.source_branch} → ${info.target_branch}`;
  }
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
  renderFileList(fileListCallbacks);
  setupEventHandlers();
  setupReviewChangedBanner();
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
