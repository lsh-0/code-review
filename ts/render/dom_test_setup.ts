// shared setup for the DOM integration tests: stand up a deno-dom document and
// install it as the global `document` the render modules use. deno-dom's WASM
// backend is sandboxed (it needs only --allow-read/--allow-net for module init,
// not FFI), matching the project's preference for the contained option. This
// is test-only and never bundled into the shipped artefact.

import { DOMParser, type HTMLDocument } from "@b-fuze/deno-dom";
import { state } from "./state.ts";

// the structural shell from `index.html` that the render layer targets by id.
const SHELL = `
<div id="app">
  <div id="review-changed-banner" class="banner hidden"></div>
  <div id="file-list">
    <div id="files"></div>
    <div id="overview-footer"></div>
  </div>
  <div id="diff-view">
    <div id="current-file-header"><h2 id="current-file-name"></h2></div>
    <div id="diff-content"></div>
  </div>
</div>
`;

// install a fresh document (with the `index.html` shell) as the global, and reset
// the view state so each test starts clean. Returns the document for queries.
export function setupDom(): HTMLDocument {
  const doc = new DOMParser().parseFromString(
    `<!DOCTYPE html><html><body>${SHELL}</body></html>`,
    "text/html",
  );
  if (!doc) {
    throw new Error("failed to parse DOM shell");
  }

  // the render layer reads `document` and constructs nodes via
  // document.createElement; point both at the deno-dom document.
  (globalThis as { document: unknown }).document = doc;

  // reset view state between tests.
  state.currentFile = "";
  state.currentUser = "";
  state.overviewActive = false;
  state.diffFiles = [];
  state.commentsCache = new Map();
  state.markedFiles = new Set();
  state.fileFilter = "";
  state.fileGrouping = "none";

  return doc;
}
