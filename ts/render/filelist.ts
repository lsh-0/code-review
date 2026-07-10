// renders the changed-file list: each file's name, its done-mark checkbox, and
// its comment-status pill, plus the review-overview entry pinned in the footer.
// The status derivation comes from the pure core (`fileCommentStatus`); this
// module builds the DOM and wires selection/marking.

import { fileCommentStatus } from "../core/comments.ts";
import { fileMatchesFilter } from "../core/filter.ts";
import { groupFiles } from "../core/grouping.ts";
import { byId, clear, el } from "../dom.ts";
import { setFileMarked as persistMarked } from "../client.ts";
import { commentsFor, state } from "./state.ts";

export interface FileListCallbacks {
  // `additive` is the ctrl/cmd modifier: a plain click selects one file, an
  // additive click toggles the file in or out of the current selection.
  onSelectFile: (filePath: string, additive: boolean) => void;
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

// build one file-list item: its comment-status pill class, done-mark checkbox
// (whose change handler is the single place marks are updated and persisted),
// and click/double-click wiring. A file in `selected` is highlighted when no
// overview, so every file of a multi-file selection is marked active.
function buildFileItem(
  filePath: string,
  selected: Set<string>,
  cb: FileListCallbacks,
): HTMLElement {
  const classes = ["file-item"];
  const status = fileStatus(filePath);
  if (status !== "none") {
    classes.push(`has-comments-${status}`);
  }
  if (selected.has(filePath) && !state.overviewActive) {
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
    // when grouped by mark, the toggle moves the file to the other group;
    // re-render so it relocates. A flat list needs no rebuild.
    if (state.fileGrouping !== "none") {
      renderFileList(cb);
    }
  });
  fileItem.appendChild(checkbox);

  fileItem.appendChild(el("div", { classes: ["file-name"], text: filePath }));

  // a ctrl/cmd-click toggles the file into the selection; a plain click selects
  // just it.
  fileItem.addEventListener(
    "click",
    (ev) => cb.onSelectFile(filePath, ev.ctrlKey || ev.metaKey),
  );

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

  return fileItem;
}

// render the whole file list and the overview footer entry. Every selected file
// (or the first, if none is selected) is highlighted; the overview entry is
// highlighted when the overview is active.
export function renderFileList(cb: FileListCallbacks): void {
  const container = byId("files");
  if (!container) {
    return;
  }
  clear(container);

  const selected = new Set(state.selectedFiles);
  // with nothing selected, highlight the first file to match the auto-select
  // below, so the list never renders with no active item.
  if (selected.size === 0 && state.diffFiles.length > 0) {
    selected.add(state.diffFiles[0].Path);
  }

  const groups = groupFiles(state.diffFiles, state.fileGrouping, {
    isMarked: (p) => state.markedFiles.has(p),
  });

  for (const group of groups) {
    const visible = group.files.filter((f) =>
      fileMatchesFilter(f.Path, state.fileFilter)
    );
    if (visible.length === 0) {
      continue;
    }
    // a labelled group (any grouper other than "none") gets a heading; the
    // single unheaded group from "none" renders as the flat list it replaces.
    if (group.label !== "") {
      container.appendChild(
        el("div", { classes: ["file-group-heading"], text: group.label }),
      );
    }
    for (const file of visible) {
      container.appendChild(buildFileItem(file.Path, selected, cb));
    }
  }

  renderOverviewEntry(cb);

  if (
    state.selectedFiles.length === 0 && !state.overviewActive &&
    state.diffFiles.length > 0
  ) {
    cb.onSelectFile(state.diffFiles[0].Path, false);
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
