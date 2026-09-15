// pure helper over the refresh banner's pending change sources. The banner has
// two independent triggers — a new commit on the source branch, and an external
// edit to the review state file — and both can be pending at once, so the
// message is derived rather than fixed. Holds no DOM or bridge dependency; the
// render layer (`main.ts`) reads it to decide what to show.

// what raised the refresh banner. `commits` means the source branch moved, so
// the diff on screen describes an older revision; `state` means another process
// (an agent, or the review CLI) edited the review's comments.
export type ChangeSource = "commits" | "state";

// the message for the refresh banner, or null when nothing is pending and the
// banner should stay hidden. The two causes call for different responses — a new
// commit invalidates the diff and the marked files, whereas a state edit is an
// agent's replies to read — so the message names which occurred rather than
// reporting a single generic sentence.
export function reviewChangedBannerText(
  sources: ReadonlySet<ChangeSource>,
): string | null {
  const commits = sources.has("commits");
  const state = sources.has("state");
  if (commits && state) {
    return "New code was committed and changes to the review have been made.";
  }
  if (commits) {
    return "New code was committed to this branch.";
  }
  if (state) {
    return "Changes to the review have been made.";
  }
  return null;
}
