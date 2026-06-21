// comment mutations and their incremental view updates. A mutation
// returns the affected file's full comment list and the recomputed file status;
// the cache is overwritten from it and only the touched thread is re-rendered,
// preserving the diff's expanded context and scroll. The overview, having no
// diff state to preserve, is rebuilt wholesale.

import type { CommentMutationResult } from "../core/types.ts";
import { getCommentsForLine } from "../core/comments.ts";
import { byId } from "../dom.ts";
import { type CommentActions, lineCommentThread } from "./comments.ts";
import { setFileStatusPill } from "./filelist.ts";
import { commentsFor, state } from "./state.ts";

// the surface callbacks a mutation needs: the comment actions threaded into
// re-rendered threads, and an overview rebuild for when the overview is active.
export interface MutationContext {
  actions: CommentActions;
  rebuildOverview: () => void;
}

// apply a mutation result incrementally. The affected file's cache is
// overwritten, then either the overview is rebuilt (if active) or the touched
// thread is patched and the file's status pill updated.
export function applyMutation(
  result: CommentMutationResult,
  ctx: MutationContext,
): void {
  state.commentsCache.set(result.file_path, result.comments);

  if (state.overviewActive) {
    ctx.rebuildOverview();
    return;
  }

  patchLineThread(result.file_path, result.line_number, ctx.actions);
  setFileStatusPill(result.file_path, result.file_status);
}

// re-render just the thread anchored at `lineNo` from the (already updated)
// cache: replace the existing thread, insert a new one after the line, or
// remove it when the line has no comments left.
function patchLineThread(
  filePath: string,
  lineNo: number,
  actions: CommentActions,
): void {
  const content = byId("diff-content");
  if (!content) {
    return;
  }

  const existing = content.querySelector(
    `.comment-thread[data-line="${lineNo}"]`,
  );
  const comments = getCommentsForLine(commentsFor(filePath), lineNo);
  const line = content.querySelector<HTMLElement>(
    `.diff-line[data-line="${lineNo}"]`,
  );

  if (comments.length === 0) {
    existing?.remove();
    // the last comment on this line is gone, so the line is no longer commented.
    line?.classList.remove("commented");
    return;
  }

  line?.classList.add("commented");

  const thread = lineCommentThread(filePath, lineNo, comments, actions);

  if (existing) {
    existing.parentNode?.replaceChild(thread, existing);
    return;
  }

  // no thread yet: insert immediately after the diff line it anchors.
  if (line) {
    line.parentNode?.insertBefore(thread, line.nextSibling);
  }
}
