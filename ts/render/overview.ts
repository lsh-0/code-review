// renders the review overview: a review-level "General feedback" section, then
// for each commented file a clickable heading and that file's commented hunks
// with threads embedded. The commented-hunk trimming lives in hunks.ts
// (overviewOnly); this module lays out the sections.

import type { Comment, CommentedFile } from "../core/types.ts";
import { rootComments, visibleComments } from "../core/comments.ts";
import { byId, clear, el } from "../dom.ts";
import { browseFile } from "../client.ts";
import { createCommentThread } from "./comments.ts";
import { renderFileHunks } from "./hunks.ts";
import { ensureFileDiffs } from "./load.ts";
import { showReviewCommentModal } from "./modals.ts";
import { selectFile } from "./view.ts";
import { state } from "./state.ts";

// render the overview into #diff-content. The commented files' hunks are
// fetched first (independently) so the real diff renders with comments embedded,
// then the sections are laid out.
export async function renderOverview(
  reviewComments: Comment[],
  files: CommentedFile[],
): Promise<void> {
  await ensureFileDiffs(files.map((f) => f.path));
  renderOverviewContent(reviewComments, files);
}

// render the overview's sections from the loaded data.
//
// The review comments and the file list arrive as arguments straight from the
// bridge, so they are filtered by source visibility here. This is the one render
// path that does not read its comments through `commentsFor`, which filters for
// every other surface. A file whose comments all belong to hidden sources is not
// listed at all, rather than listed with an empty body.
function renderOverviewContent(
  reviewComments: Comment[],
  files: CommentedFile[],
): void {
  const content = byId("diff-content");
  if (!content) {
    return;
  }
  clear(content);

  content.appendChild(
    overviewReviewSection(visibleComments(reviewComments, state.hiddenSources)),
  );

  for (const file of visibleCommentedFiles(files)) {
    const section = el("div", { classes: ["overview-file"] });
    section.appendChild(overviewFileHeader(file.path));
    // only this file's commented hunks, read-only, threads embedded.
    renderFileHunks(section, file.path, true);
    content.appendChild(section);
  }
}

// the commented files that still have a visible comment. A file whose feedback all
// belongs to hidden sources has nothing to show, so it drops out of the overview.
function visibleCommentedFiles(files: CommentedFile[]): CommentedFile[] {
  if (state.hiddenSources.size === 0) {
    return files;
  }
  return files.filter(
    (f) => visibleComments(f.comments, state.hiddenSources).length > 0,
  );
}

// the review-level feedback section: a heading, an "add comment" control, and
// any existing review comments threaded (routed to the review level via the
// empty file path).
function overviewReviewSection(reviewComments: Comment[]): HTMLElement {
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
      onClick: () => showReviewCommentModal(),
    }),
  );

  if (reviewComments.length > 0) {
    // review comments are keyed under the empty path; their handlers pass "" as
    // the file, routing status/reply/delete to the review level.
    section.appendChild(createCommentThread("", rootComments(reviewComments)));
  }

  return section;
}

// an overview file section header: the filename (linking to that file's page)
// and a "browse" button to open it in the OS-preferred application.
function overviewFileHeader(filePath: string): HTMLElement {
  const header = el("div", { classes: ["overview-file-header"] });

  header.appendChild(
    el("button", {
      classes: ["overview-file-name"],
      text: filePath,
      attrs: { title: "Open this file's page" },
      onClick: () => void selectFile(filePath),
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
