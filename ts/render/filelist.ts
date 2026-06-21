// renders the changed-file list: each file's name, its done-mark checkbox, and
// its comment-status pill, plus the review-overview entry pinned in the footer.
// The status derivation comes from the pure core (`fileCommentStatus`); this
// module builds the DOM and wires selection/marking.

import { fileCommentStatus } from "../core/comments.ts";
import { byId, clear, el } from "../dom.ts";
import { setFileMarked as persistMarked } from "../client.ts";
import { commentsFor, state } from "./state.ts";

export interface FileListCallbacks {
  onSelectFile: (filePath: string) => void;
  onSelectOverview: () => void;
}

// the comment-status pill class suffix for a file, from its cached comments.
// "none" when the file has no comments.
function fileStatus(filePath: string): string {
  const comments = commentsFor(filePath);
  if (comments.length === 0) {
    return "none";
  }
  return fileCommentStatus(comments);
}

// render the whole file list and the overview footer entry. The selected file
// (or the first, if none) is highlighted; the overview entry is highlighted
// when the overview is active.
export function renderFileList(cb: FileListCallbacks): void {
  const container = byId("files");
  if (!container) {
    return;
  }
  clear(container);

  let selectedFile = state.currentFile;
  if (selectedFile === "" && state.diffFiles.length > 0) {
    selectedFile = state.diffFiles[0].Path;
  }

  for (const file of state.diffFiles) {
    const filePath = file.Path;
    const classes = ["file-item"];
    const status = fileStatus(filePath);
    if (status !== "none") {
      classes.push(`has-comments-${status}`);
    }
    if (filePath === selectedFile && !state.overviewActive) {
      classes.push("active");
    }

    const fileItem = el("div", { classes });
    fileItem.dataset.path = filePath;

    const checkbox = el("input", { classes: ["file-marked"] });
    checkbox.setAttribute("type", "checkbox");
    checkbox.checked = state.markedFiles.has(filePath);
    // clicking the checkbox must not bubble to the file-item's selection.
    checkbox.addEventListener("click", (ev) => ev.stopPropagation());
    checkbox.addEventListener("change", () => {
      const marked = checkbox.checked;
      if (marked) {
        state.markedFiles.add(filePath);
      } else {
        state.markedFiles.delete(filePath);
      }
      void persistMarked(filePath, marked);
    });
    fileItem.appendChild(checkbox);

    fileItem.appendChild(el("div", { classes: ["file-name"], text: filePath }));

    fileItem.addEventListener("click", () => cb.onSelectFile(filePath));

    // the double-click below would otherwise form a word selection on the
    // filename; cancelling selectstart stops that highlight flash.
    fileItem.addEventListener("selectstart", (ev) => ev.preventDefault());

    // double-clicking toggles the done checkbox (the constituent single clicks
    // select the file first). Drive the checkbox so its change handler stays the
    // single place that updates and persists.
    fileItem.addEventListener("dblclick", () => {
      checkbox.checked = !checkbox.checked;
      checkbox.dispatchEvent(new Event("change"));
    });

    container.appendChild(fileItem);
  }

  renderOverviewEntry(cb);

  if (
    state.currentFile === "" && !state.overviewActive &&
    state.diffFiles.length > 0
  ) {
    cb.onSelectFile(state.diffFiles[0].Path);
  }
}

// render the review-overview entry into its footer, pinned at the bottom of the
// file list so it stays put while the list scrolls.
function renderOverviewEntry(cb: FileListCallbacks): void {
  const footer = byId("overview-footer");
  if (!footer) {
    return;
  }
  clear(footer);

  const classes = ["file-item", "overview-item"];
  if (state.overviewActive) {
    classes.push("active");
  }
  const entry = el("div", {
    classes,
    onClick: () => cb.onSelectOverview(),
    children: [el("div", { classes: ["file-name"], text: "Review overview" })],
  });
  footer.appendChild(entry);
}

// update one file's status pill without rebuilding the list, preserving the
// rest. Used by incremental comment mutations.
export function setFileStatusPill(filePath: string, status: string): void {
  const items = document.querySelectorAll<HTMLElement>(".file-item");
  for (const item of items) {
    if (item.dataset.path !== filePath) {
      continue;
    }
    for (const s of ["active", "resolved", "ignored"]) {
      item.classList.remove(`has-comments-${s}`);
    }
    if (status !== "none") {
      item.classList.add(`has-comments-${status}`);
    }
    return;
  }
}
