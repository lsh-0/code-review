// the frontend entry point: wires the toolbar, modals, zoom, keyboard and
// wheel handlers, and the external-change banners, then loads the initial
// view. Selection and region rendering live in render/view.ts; this module is
// only the page-level wiring. Bundled to `assets/review.js` and loaded by
// `index.html`.

import * as api from "./client.ts";
import {
  type ChangeSource,
  reviewChangedBannerText,
} from "./core/notification.ts";
import { byId, el, requireId } from "./dom.ts";
import { currentFile, state } from "./render/state.ts";
import { clampPaneWidth } from "./core/panes.ts";
import { renderFileList } from "./render/filelist.ts";
import {
  closeFind,
  findIsOpen,
  findNext,
  findPrev,
  openFind,
} from "./render/find.ts";
import { groupers } from "./core/grouping.ts";
import {
  hideCommentModal,
  hideEditCommentModal,
  hideReplyModal,
  saveComment,
  saveReply,
  updateComment,
} from "./render/modals.ts";
import {
  loadCommentedFiles,
  loadDiffFiles,
  recomputeDiff,
} from "./render/load.ts";
import {
  fileListCallbacks,
  renderFileDirtyBanner,
  renderSelection,
  renderWorkingTreeBanner,
} from "./render/view.ts";
import { renderOverview } from "./render/overview.ts";

const zoomMin = 0.5;
const zoomMax = 3.0;
const zoomStep = 0.1;

// recompute the diff and reload state, then re-render the file list and the
// visible surface. Window state collapses to defaults (loadDiffFiles clears
// it), per the refresh rule.
async function performFullRefresh(): Promise<void> {
  await recomputeDiff();
  await loadDiffFiles();
  await refreshWorkingTreeStatus();
  renderFileList(fileListCallbacks);
  if (state.overviewActive) {
    const { reviewComments, files } = await loadCommentedFiles();
    await renderOverview(reviewComments, files);
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

// fetch the working-tree status into state and render the page-wide banner.
// Called on load and after every full refresh, since a refresh may follow a
// new commit that changed what is dirty. A fetch failure leaves the previous
// state and banner untouched rather than clearing a still-valid warning.
async function refreshWorkingTreeStatus(): Promise<void> {
  try {
    state.workingTreeStatus = await api.getWorkingTreeStatus();
  } catch {
    return;
  }
  renderWorkingTreeBanner();
  renderFileDirtyBanner(currentFile());
}

// which changes are pending on the refresh banner. A watcher suspends once it
// has alerted, so this only ever accumulates a second entry when the other
// watcher alerts before the reviewer clears the banner.
const pendingChanges = new Set<ChangeSource>();

// raise the refresh banner for a newly detected change and suspend the watcher
// that reported it: it has done its job, and polling on would only re-report
// what the reviewer has already been shown.
function raiseReviewChangedBanner(source: ChangeSource): void {
  pendingChanges.add(source);
  const text = byId("review-changed-banner-text");
  if (text) {
    text.textContent = reviewChangedBannerText(pendingChanges) ?? "";
  }
  byId("review-changed-banner")?.classList.remove("hidden");
  void api.suspendWatch(source);
}

// clear the banner, resuming every watcher it was speaking for. Both the refresh
// and the dismiss control come here: refreshing absorbs the change and
// dismissing declines it, but either way the reviewer has seen it, so detection
// must start again rather than stay silent for the rest of the session.
function clearReviewChangedBanner(): void {
  byId("review-changed-banner")?.classList.add("hidden");
  for (const source of pendingChanges) {
    void api.resumeWatch(source);
  }
  pendingChanges.clear();
}

// wire the external-change banner: the backend's `review:changed` and
// `commits:changed` events raise it, naming which of the two occurred; its
// buttons refresh or dismiss, and both clear it.
function setupReviewChangedBanner(): void {
  const runtime =
    (globalThis as { runtime?: { EventsOn(ev: string, cb: () => void): void } })
      .runtime;
  runtime?.EventsOn("review:changed", () => raiseReviewChangedBanner("state"));
  runtime?.EventsOn(
    "commits:changed",
    () => raiseReviewChangedBanner("commits"),
  );

  byId("banner-refresh-btn")?.addEventListener("click", () => {
    clearReviewChangedBanner();
    void triggerRefresh();
  });
  byId("banner-dismiss-btn")?.addEventListener(
    "click",
    () => clearReviewChangedBanner(),
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
      // the find bar handles Escape from its own input; this closes it when the
      // focus has moved elsewhere (e.g. after clicking into the diff).
      if (findIsOpen()) {
        closeFind();
      }
    }
    // Ctrl-f opens the in-page find bar. WebKitGTK does not wire up the native
    // browser find, so it is implemented in the render layer.
    if (ev.ctrlKey && (key === "f" || key === "F")) {
      ev.preventDefault();
      openFind();
      return;
    }
    // F3 / Shift+F3 step the find like a native browser: next / previous when the
    // bar is open, and opening it (a "find again") when it is closed.
    if (key === "F3") {
      ev.preventDefault();
      if (findIsOpen()) {
        if (ev.shiftKey) {
          findPrev();
        } else {
          findNext();
        }
      } else {
        openFind();
      }
      return;
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
  byId("file-dirty-difftool-btn")?.addEventListener("click", () => {
    const filePath = currentFile();
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
