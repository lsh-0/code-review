// the add/edit/reply comment modals and the save handlers that call the client
// and apply the returned mutation. The backend captures a new comment's anchor
// context itself; the modal transient state lives in `state.modal`.

import { byId } from "../dom.ts";
import * as api from "../client.ts";
import { applyMutation } from "./mutate.ts";
import { state } from "./state.ts";

function showModal(modalID: string, inputID: string, value: string): void {
  const modal = byId(modalID);
  const input = byId<HTMLTextAreaElement>(inputID);
  if (!modal || !input) {
    return;
  }
  input.value = value;
  modal.classList.add("active");
  // guarded: the deno-dom test shim does not implement focus.
  input.focus?.();
}

function hideModal(modalID: string): void {
  byId(modalID)?.classList.remove("active");
}

export function showCommentModal(filePath: string, lineNumber: number): void {
  state.modal.file = filePath;
  state.modal.lineNumber = lineNumber;
  state.modal.reviewCommentMode = false;
  state.modal.fileCommentMode = false;
  showModal("comment-modal", "comment-input", "");
}

// open the comment modal in review mode: the save stores overall feedback not
// anchored to any file or line.
export function showReviewCommentModal(): void {
  state.modal.reviewCommentMode = true;
  state.modal.fileCommentMode = false;
  showModal("comment-modal", "comment-input", "");
}

// open the comment modal in file mode: the save attaches to `filePath` but to
// no line (the top-level file comment).
export function showFileCommentModal(filePath: string): void {
  state.modal.file = filePath;
  state.modal.reviewCommentMode = false;
  state.modal.fileCommentMode = true;
  showModal("comment-modal", "comment-input", "");
}

export function hideCommentModal(): void {
  hideModal("comment-modal");
}

// save the comment-modal's content: a review-level comment when in review mode,
// otherwise a line comment (the backend captures its anchor context).
export async function saveComment(): Promise<void> {
  const input = byId<HTMLTextAreaElement>("comment-input");
  const content = input?.value ?? "";
  if (content === "") {
    return;
  }

  if (state.modal.reviewCommentMode) {
    const result = await api.addReviewComment(content);
    hideCommentModal();
    applyMutation(result);
    return;
  }

  if (state.modal.fileCommentMode) {
    const result = await api.addFileComment(state.modal.file, content);
    hideCommentModal();
    applyMutation(result);
    return;
  }

  const result = await api.addComment(
    state.modal.file,
    content,
    state.modal.lineNumber,
  );
  hideCommentModal();
  applyMutation(result);
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
  applyMutation(result);
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
  applyMutation(result);
}

// the status/delete actions on a comment, each calling the client then applying
// the result. Wired straight into the threads by the render layer.
export async function resolveComment(
  filePath: string,
  commentID: string,
): Promise<void> {
  applyMutation(await api.resolveComment(filePath, commentID));
}

export async function ignoreComment(
  filePath: string,
  commentID: string,
): Promise<void> {
  applyMutation(await api.ignoreComment(filePath, commentID));
}

export async function reactivateComment(
  filePath: string,
  commentID: string,
): Promise<void> {
  applyMutation(await api.reactivateComment(filePath, commentID));
}

export async function deleteComment(
  filePath: string,
  commentID: string,
): Promise<void> {
  applyMutation(await api.deleteComment(filePath, commentID));
}
