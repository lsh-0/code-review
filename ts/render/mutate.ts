// comment mutations land here: the affected surface's cache is overwritten
// from the mutation result and its region re-rendered from state. Window state
// (growth, revealed lines) survives the re-render because it lives in state,
// not the DOM; a comment placed outside the current windows gains its island
// during the region render.

import type { CommentMutationResult } from "../core/types.ts";
import { setFileStatusPill } from "./filelist.ts";
import { state } from "./state.ts";
import { rebuildOverview, rerenderFileRegion } from "./view.ts";

// apply a mutation result: overwrite the surface's comment cache, then rebuild
// the overview (if active) or re-render the file's region and refresh its
// status pill. A review-level mutation (empty path) outside the overview has
// no region to touch.
export function applyMutation(result: CommentMutationResult): void {
  state.commentsCache.set(result.file_path, result.comments);

  if (state.overviewActive) {
    rebuildOverview();
    return;
  }
  if (result.file_path === "") {
    return;
  }

  setFileStatusPill(result.file_path, result.file_status);
  rerenderFileRegion(result.file_path);
}
