// renders the review overview: a review-level "General feedback" section, then
// for each commented file a clickable heading and that file's commented hunks
// with threads embedded. The commented-hunk trimming lives in hunks.ts
// (overviewOnly); this module lays out the sections.

import type { Comment, CommentedFile } from "../core/types.ts";
import { rootComments } from "../core/comments.ts";
import { byId, clear, el } from "../dom.ts";
import { browseFile } from "../client.ts";
import { type CommentActions, createCommentThread } from "./comments.ts";
import { type DiffCallbacks, renderFileHunks } from "./hunks.ts";
import { ensureFileDiffs } from "./load.ts";

export interface OverviewCallbacks {
  diff: DiffCallbacks;
  actions: CommentActions;
  onSelectFile: (filePath: string) => void;
  onAddReviewComment: () => void;
}

// render the overview into #diff-content. The commented files' hunks are
// fetched first (independently) so the real diff renders with comments embedded,
// then the sections are laid out.
export async function renderOverview(
  reviewComments: Comment[],
  files: CommentedFile[],
  cb: OverviewCallbacks,
): Promise<void> {
  await ensureFileDiffs(files.map((f) => f.path));
  renderOverviewContent(reviewComments, files, cb);
}

function renderOverviewContent(
  reviewComments: Comment[],
  files: CommentedFile[],
  cb: OverviewCallbacks,
): void {
  const content = byId("diff-content");
  if (!content) {
    return;
  }
  clear(content);

  content.appendChild(overviewReviewSection(reviewComments, cb));

  for (const file of files) {
    const section = el("div", { classes: ["overview-file"] });
    section.appendChild(overviewFileHeader(file.path, cb));
    // only this file's commented hunks, read-only, threads embedded.
    renderFileHunks(section, file.path, cb.diff, true);
    content.appendChild(section);
  }
}

// the review-level feedback section: a heading, an "add comment" control, and
// any existing review comments threaded (routed to the review level via the
// empty file path).
function overviewReviewSection(
  reviewComments: Comment[],
  cb: OverviewCallbacks,
): HTMLElement {
  const section = el("div", { classes: ["overview-file", "overview-review"] });

  const heading = el("div", { classes: ["overview-file-header"] });
  heading.appendChild(
    el("span", {
      classes: ["overview-file-name"],
      text: "General review comments",
    }),
  );
  section.appendChild(heading);

  section.appendChild(
    el("button", {
      classes: ["overview-add-comment"],
      text: "Add comment",
      onClick: () => cb.onAddReviewComment(),
    }),
  );

  if (reviewComments.length > 0) {
    // review comments are keyed under the empty path; their handlers pass "" as
    // the file, routing status/reply/delete to the review level.
    section.appendChild(
      createCommentThread("", rootComments(reviewComments), cb.actions),
    );
  }

  return section;
}

// an overview file section header: the filename (linking to that file's page)
// and a "browse" button to open it in the OS-preferred application.
function overviewFileHeader(
  filePath: string,
  cb: OverviewCallbacks,
): HTMLElement {
  const header = el("div", { classes: ["overview-file-header"] });

  header.appendChild(
    el("button", {
      classes: ["overview-file-name"],
      text: filePath,
      attrs: { title: "Open this file's page" },
      onClick: () => cb.onSelectFile(filePath),
    }),
  );

  header.appendChild(
    el("button", {
      classes: ["browse-link"],
      text: "browse",
      attrs: { title: "Open this file in the preferred application" },
      onClick: () => {
        void browseFile(filePath).catch(() =>
          globalThis.alert(`Could not open ${filePath}`)
        );
      },
    }),
  );

  return header;
}
