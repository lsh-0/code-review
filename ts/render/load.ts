// data-loading helpers that bridge the client to the view state: fetching the
// diff metadata, priming the comment caches, loading marked files, and lazily
// fetching a file's hunks on first use. These mutate `state` (the side effect)
// but contain no view logic; rendering reads the primed state afterwards.

import type { Comment, CommentedFile } from "../core/types.ts";
import * as api from "../client.ts";
import { diffFile, diffFileNeedsFetch, state } from "./state.ts";

// fetch the diff metadata into `state.diffFiles`, then prime comments and marked
// files. `GetDiffFiles` returns no hunks, so each entry's hunks are fetched
// lazily on first selection; clearing first forces a refresh to re-fetch rather
// than serve stale hunks.
export async function loadDiffFiles(): Promise<void> {
  const metas = await api.getDiffFiles();
  state.diffFiles = metas.map((m) => ({
    Path: m.Path,
    Binary: m.Binary,
    Hunks: [],
  }));
  await loadAllComments();
  await loadMarkedFiles();
}

// prime `commentsCache` for every commented file in one bridge call. Files
// absent from the result simply have status "none".
export async function loadAllComments(): Promise<void> {
  await loadCommentedFiles();
}

// fetch every commented file and the review-level comments, priming the cache.
// Review comments are keyed under the empty path so the shared comment handlers
// resolve their replies and route status changes to the review level. Returns
// the loaded data for the overview to render.
export async function loadCommentedFiles(): Promise<{
  reviewComments: Comment[];
  files: CommentedFile[];
}> {
  const files = await api.getCommentedFiles();
  for (const f of files) {
    state.commentsCache.set(f.path, f.comments);
  }
  const reviewComments = await api.getReviewComments();
  state.commentsCache.set("", reviewComments);
  return { reviewComments, files };
}

// load the marked-file set into `state.markedFiles`.
export async function loadMarkedFiles(): Promise<void> {
  const paths = await api.getMarkedFiles();
  state.markedFiles = new Set(paths);
}

// load one file's comments into the cache.
export async function loadComments(filePath: string): Promise<void> {
  const comments = await api.getComments(filePath);
  state.commentsCache.set(filePath, comments);
}

// ensure the in-memory diff entry for `filePath` has its hunks, fetching them
// when absent. Metadata-only entries get their hunks on first selection; binary
// files never have hunks. A missing entry or fetch error resolves quietly so
// the caller still renders.
export async function ensureFileDiff(filePath: string): Promise<void> {
  const file = diffFile(filePath);
  if (!file || !diffFileNeedsFetch(file)) {
    return;
  }
  try {
    const fetched = await api.getFileDiff(filePath);
    if (fetched) {
      file.Hunks = fetched.Hunks;
      file.Binary = fetched.Binary;
    }
  } catch {
    // leave the entry as-is; the caller renders the empty/binary path.
  }
}

// ensure the hunks of every path are loaded. Fetches run concurrently.
export async function ensureFileDiffs(paths: string[]): Promise<void> {
  await Promise.all(paths.map((p) => ensureFileDiff(p)));
}

// reload review state from the JSON (no git), refreshing the comment caches.
export async function reloadReview(): Promise<void> {
  await api.reloadReview();
}

// recompute the diff from git, then reload review state.
export async function recomputeDiff(): Promise<void> {
  await api.recomputeDiff();
  await reloadReview();
}
