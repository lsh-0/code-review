// the add/edit/reply comment modals and the save handlers that call the client
// and apply the returned mutation. Line context for a new comment comes from
// the pure `getLineContext`; the modal transient state lives in `state.modal`.

import { getLineContext } from "../core/overview.ts";
import { byId } from "../dom.ts";
import * as api from "../client.ts";
import { applyMutation, type MutationContext } from "./mutate.ts";
import { diffFile, state } from "./state.ts";

// the modal handlers need a mutation context to apply results, supplied once at
// wire-up.
let mutationCtx: MutationContext;

export function initModals(ctx: MutationContext): void {
  mutationCtx = ctx;
}

function showModal(modalID: string, inputID: string, value: string): void {
  const modal = byId(modalID);
  const input = byId<HTMLTextAreaElement>(inputID);
  if (!modal || !input) {
    return;
  }
  input.value = value;
  modal.classList.add("active");
  input.focus();
}

function hideModal(modalID: string): void {
  byId(modalID)?.classList.remove("active");
}

// open the add-comment modal for a line in the current file.
export function showCommentModal(filePath: string, lineNumber: number): void {
  state.modal.reviewCommentMode = false;
  state.modal.file = filePath;
  state.modal.lineNumber = lineNumber;
  showModal("comment-modal", "comment-input", "");
}

// open the add-comment modal for a review-level comment (overall feedback, no
// file or line anchor).
export function showReviewCommentModal(): void {
  state.modal.reviewCommentMode = true;
  showModal("comment-modal", "comment-input", "");
}

export function hideCommentModal(): void {
  hideModal("comment-modal");
}

// save the comment-modal's content: a review-level comment when in review mode,
// otherwise a line comment carrying its surrounding context.
export async function saveComment(): Promise<void> {
  const input = byId<HTMLTextAreaElement>("comment-input");
  const content = input?.value ?? "";
  if (content === "") {
    return;
  }

  if (state.modal.reviewCommentMode) {
    const result = await api.addReviewComment(content);
    hideCommentModal();
    applyMutation(result, mutationCtx);
    return;
  }

  const { context, offset } = getLineContext(
    diffFile(state.modal.file),
    state.modal.lineNumber,
  );
  const result = await api.addComment(
    state.modal.file,
    content,
    state.modal.lineNumber,
    context,
    offset,
  );
  hideCommentModal();
  applyMutation(result, mutationCtx);
}

export function showEditCommentModal(
  filePath: string,
  commentID: string,
  content: string,
): void {
  state.modal.file = filePath;
  state.modal.commentID = commentID;
  showModal("edit-comment-modal", "edit-comment-input", content);
}

export function hideEditCommentModal(): void {
  hideModal("edit-comment-modal");
}

export async function updateComment(): Promise<void> {
  const input = byId<HTMLTextAreaElement>("edit-comment-input");
  const content = input?.value ?? "";
  if (content === "") {
    return;
  }
  const result = await api.updateComment(
    state.modal.file,
    state.modal.commentID,
    content,
  );
  hideEditCommentModal();
  applyMutation(result, mutationCtx);
}

export function showReplyModal(filePath: string, commentID: string): void {
  state.modal.file = filePath;
  state.modal.replyID = commentID;
  showModal("reply-comment-modal", "reply-comment-input", "");
}

export function hideReplyModal(): void {
  hideModal("reply-comment-modal");
}

export async function saveReply(): Promise<void> {
  const input = byId<HTMLTextAreaElement>("reply-comment-input");
  const content = input?.value ?? "";
  if (content === "") {
    return;
  }
  const result = await api.addReply(
    state.modal.file,
    state.modal.replyID,
    content,
  );
  hideReplyModal();
  applyMutation(result, mutationCtx);
}

// the status/delete actions on a comment, each calling the client then applying
// the result. These back the CommentActions wired into every thread.
export async function resolveComment(
  filePath: string,
  commentID: string,
): Promise<void> {
  applyMutation(await api.resolveComment(filePath, commentID), mutationCtx);
}

export async function ignoreComment(
  filePath: string,
  commentID: string,
): Promise<void> {
  applyMutation(await api.ignoreComment(filePath, commentID), mutationCtx);
}

export async function reactivateComment(
  filePath: string,
  commentID: string,
): Promise<void> {
  applyMutation(await api.reactivateComment(filePath, commentID), mutationCtx);
}

export async function deleteComment(
  filePath: string,
  commentID: string,
): Promise<void> {
  applyMutation(await api.deleteComment(filePath, commentID), mutationCtx);
}
