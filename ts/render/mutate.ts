// comment mutations land here: the affected surface's cache is overwritten
// from the mutation result and its region re-rendered from state. Window state
// (growth, revealed lines) survives the re-render because it lives in state,
// not the DOM; a comment placed outside the current windows gains its island
// during the region render.

import type { CommentMutationResult } from "../core/types.ts";
import { fileCommentStatus } from "../core/comments.ts";
import { setFileStatusPill } from "./filelist.ts";
import { commentsFor, state } from "./state.ts";
import { rebuildOverview, rerenderFileRegion } from "./view.ts";

// apply a mutation result: overwrite the surface's comment cache, then rebuild
// the overview (if active) or re-render the file's region and refresh its
// status pill. A review-level mutation (empty path) outside the overview has
// no region to touch.
//
// The pill is recomputed from the visible cache rather than taken from the
// result's `file_status`: the backend computes that over every comment and has no
// knowledge of which sources the reviewer has hidden, so using it would restore a
// pill for hidden feedback on the next mutation. The two agree whenever nothing is
// hidden.
export function applyMutation(result: CommentMutationResult): void {
  state.commentsCache.set(result.file_path, result.comments);

  if (state.overviewActive) {
    rebuildOverview();
    return;
  }
  if (result.file_path === "") {
    return;
  }

  setFileStatusPill(result.file_path, visibleFileStatus(result.file_path));
  rerenderFileRegion(result.file_path);
}

// the file's comment-status pill from its visible comments, matching how the file
// list derives it. "none" when no visible comment remains.
function visibleFileStatus(filePath: string): string {
  const comments = commentsFor(filePath);
  return comments.length === 0 ? "none" : fileCommentStatus(comments);
}
