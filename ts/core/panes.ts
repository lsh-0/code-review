// pure geometry for the file-list/diff divider. The file-list width is held
// between 5% and 95% of the window so neither pane can be dragged away entirely
// — a sliver of each always remains visible.

export const paneMinFraction = 0.05;
export const paneMaxFraction = 0.95;

// clamp a desired file-list width (px) to the allowed fraction of the window.
export function clampPaneWidth(widthPx: number, windowWidthPx: number): number {
  const min = windowWidthPx * paneMinFraction;
  const max = windowWidthPx * paneMaxFraction;
  return Math.min(max, Math.max(min, widthPx));
}
