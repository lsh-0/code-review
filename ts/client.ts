// the typed RPC client over the Wails bridge. It wraps `window.go.main.App.*`
// directly rather than importing the Wails-generated module, so the bundle has
// no dependency on the generated file path or Wails' runtime module; the
// not-ready guard lives here in one place. Each bound `App` method gets one
// typed async method, and the JSON-string results the backend returns are
// decoded into the wire types.

import type {
  Comment,
  CommentedFile,
  CommentMutationResult,
  DiffFile,
  DiffFileMeta,
  FileLinesResult,
  WorkingTreeStatus,
} from "./core/types.ts";

// the shape of the Wails-bound App object on `window.go.main.App`. Every bound
// method is a function returning a Promise; the ones that return data resolve a
// JSON string, the void ones resolve undefined.
interface BoundApp {
  GetReviewInfo(): Promise<string>;
  GetStatePrompt(): Promise<string>;
  GetDiffFiles(): Promise<string>;
  GetFileDiff(path: string): Promise<string>;
  GetWorkingTreeStatus(): Promise<string>;
  GetFileLines(
    path: string,
    startNew: number,
    endNew: number,
    oldOffset: number,
  ): Promise<string>;
  GetComments(path: string): Promise<string>;
  GetCommentedFiles(): Promise<string>;
  GetReviewComments(): Promise<string>;
  GetMarkedFiles(): Promise<string>;
  SetFileMarked(path: string, marked: boolean): Promise<void>;
  BrowseFile(path: string): Promise<void>;
  AddComment(
    path: string,
    content: string,
    lineNumber: number,
    context: string[],
    offset: number,
  ): Promise<string>;
  AddReviewComment(content: string): Promise<string>;
  UpdateComment(
    path: string,
    commentID: string,
    content: string,
  ): Promise<string>;
  AddReply(path: string, commentID: string, content: string): Promise<string>;
  ResolveComment(path: string, commentID: string): Promise<string>;
  IgnoreComment(path: string, commentID: string): Promise<string>;
  ReactivateComment(path: string, commentID: string): Promise<string>;
  DeleteComment(path: string, commentID: string): Promise<string>;
  RecomputeDiff(): Promise<void>;
  ReloadReview(): Promise<void>;
}

interface WailsGlobal {
  go?: { main?: { App?: BoundApp } };
}

// resolve the bound backend App, or null if the bridge is not yet ready. Every
// call goes through here so the not-ready guard lives in one place.
function boundApp(): BoundApp | null {
  const g = globalThis as unknown as WailsGlobal;
  return g.go?.main?.App ?? null;
}

// a description of what is currently present on the global, for diagnosing a
// bridge that never becomes ready: reports which level of `window.go.main.App`
// is missing, and whether the Wails runtime is present at all.
export function bridgeDiagnostic(): string {
  const g = globalThis as unknown as {
    go?: { main?: { App?: unknown } };
    runtime?: unknown;
  };
  const has = (b: boolean) => (b ? "yes" : "no");
  const goKeys = g.go
    ? Object.keys(g.go).join(",") || "(empty)"
    : "(no window.go)";
  return [
    `window.go=${has(!!g.go)}`,
    `window.go.main=${has(!!g.go?.main)}`,
    `window.go.main.App=${has(!!g.go?.main?.App)}`,
    `window.runtime=${has(!!g.runtime)}`,
    `go keys=[${goKeys}]`,
  ].join(" ");
}

// thrown when a bridge call is attempted before the backend is bound. Callers
// that must degrade gracefully (e.g. initial render before the bridge settles)
// catch this; most callers let it propagate to the console.
export class BridgeNotReadyError extends Error {
  constructor(method: string) {
    super(`backend bridge not ready for ${method}`);
    this.name = "BridgeNotReadyError";
  }
}

// the review info returned by `GetReviewInfo` (decoded from its JSON string).
export interface ReviewInfo {
  repo_path: string;
  source_branch: string;
  target_branch: string;
  current_user: string;
}

// reviewInfo / statePrompt etc. each call one bound method and decode its
// result. The decode is centralised in `callJSON`; void methods use `callVoid`.

async function callJSON<T>(
  method: keyof BoundApp,
  invoke: (app: BoundApp) => Promise<string>,
): Promise<T> {
  const app = boundApp();
  if (!app) {
    throw new BridgeNotReadyError(method);
  }
  const raw = await invoke(app);
  return JSON.parse(raw) as T;
}

async function callVoid(
  method: keyof BoundApp,
  invoke: (app: BoundApp) => Promise<void>,
): Promise<void> {
  const app = boundApp();
  if (!app) {
    throw new BridgeNotReadyError(method);
  }
  await invoke(app);
}

// GetStatePrompt returns a plain (non-JSON) string, so it is read directly.
async function callString(
  method: keyof BoundApp,
  invoke: (app: BoundApp) => Promise<string>,
): Promise<string> {
  const app = boundApp();
  if (!app) {
    throw new BridgeNotReadyError(method);
  }
  return await invoke(app);
}

export const isReady = (): boolean => boundApp() !== null;

// resolve once the Wails bridge has injected `window.go.main.App`. The runtime
// loads asynchronously and is not guaranteed to be present at the page's `load`
// event, so initialisation must wait for it rather than assume it. Polls at a
// short interval up to `timeoutMs`; resolves true when ready, false on timeout.
export function waitForBridge(
  timeoutMs = 5000,
  intervalMs = 25,
): Promise<boolean> {
  return new Promise((resolve) => {
    if (isReady()) {
      resolve(true);
      return;
    }
    let waited = 0;
    const timer = setInterval(() => {
      if (isReady()) {
        clearInterval(timer);
        resolve(true);
        return;
      }
      waited += intervalMs;
      if (waited >= timeoutMs) {
        clearInterval(timer);
        resolve(false);
      }
    }, intervalMs);
  });
}

export const getReviewInfo = (): Promise<ReviewInfo> =>
  callJSON("GetReviewInfo", (a) => a.GetReviewInfo());

export const getStatePrompt = (): Promise<string> =>
  callString("GetStatePrompt", (a) => a.GetStatePrompt());

export const getDiffFiles = (): Promise<DiffFileMeta[]> =>
  callJSON("GetDiffFiles", (a) => a.GetDiffFiles());

// GetFileDiff yields a null result for an unknown path; JSON.parse("null") is
// null, so the return type admits it.
export const getFileDiff = (path: string): Promise<DiffFile | null> =>
  callJSON("GetFileDiff", (a) => a.GetFileDiff(path));

export const getWorkingTreeStatus = (): Promise<WorkingTreeStatus> =>
  callJSON("GetWorkingTreeStatus", (a) => a.GetWorkingTreeStatus());

export const getFileLines = (
  path: string,
  startNew: number,
  endNew: number,
  oldOffset: number,
): Promise<FileLinesResult> =>
  callJSON(
    "GetFileLines",
    (a) => a.GetFileLines(path, startNew, endNew, oldOffset),
  );

export const getComments = (path: string): Promise<Comment[]> =>
  callJSON("GetComments", (a) => a.GetComments(path));

export const getCommentedFiles = (): Promise<CommentedFile[]> =>
  callJSON("GetCommentedFiles", (a) => a.GetCommentedFiles());

export const getReviewComments = (): Promise<Comment[]> =>
  callJSON("GetReviewComments", (a) => a.GetReviewComments());

export const getMarkedFiles = (): Promise<string[]> =>
  callJSON("GetMarkedFiles", (a) => a.GetMarkedFiles());

export const setFileMarked = (path: string, marked: boolean): Promise<void> =>
  callVoid("SetFileMarked", (a) => a.SetFileMarked(path, marked));

export const browseFile = (path: string): Promise<void> =>
  callVoid("BrowseFile", (a) => a.BrowseFile(path));

export const addComment = (
  path: string,
  content: string,
  lineNumber: number,
  context: string[],
  offset: number,
): Promise<CommentMutationResult> =>
  callJSON(
    "AddComment",
    (a) => a.AddComment(path, content, lineNumber, context, offset),
  );

export const addReviewComment = (
  content: string,
): Promise<CommentMutationResult> =>
  callJSON("AddReviewComment", (a) => a.AddReviewComment(content));

export const updateComment = (
  path: string,
  commentID: string,
  content: string,
): Promise<CommentMutationResult> =>
  callJSON("UpdateComment", (a) => a.UpdateComment(path, commentID, content));

export const addReply = (
  path: string,
  commentID: string,
  content: string,
): Promise<CommentMutationResult> =>
  callJSON("AddReply", (a) => a.AddReply(path, commentID, content));

export const resolveComment = (
  path: string,
  commentID: string,
): Promise<CommentMutationResult> =>
  callJSON("ResolveComment", (a) => a.ResolveComment(path, commentID));

export const ignoreComment = (
  path: string,
  commentID: string,
): Promise<CommentMutationResult> =>
  callJSON("IgnoreComment", (a) => a.IgnoreComment(path, commentID));

export const reactivateComment = (
  path: string,
  commentID: string,
): Promise<CommentMutationResult> =>
  callJSON("ReactivateComment", (a) => a.ReactivateComment(path, commentID));

export const deleteComment = (
  path: string,
  commentID: string,
): Promise<CommentMutationResult> =>
  callJSON("DeleteComment", (a) => a.DeleteComment(path, commentID));

export const recomputeDiff = (): Promise<void> =>
  callVoid("RecomputeDiff", (a) => a.RecomputeDiff());

export const reloadReview = (): Promise<void> =>
  callVoid("ReloadReview", (a) => a.ReloadReview());
