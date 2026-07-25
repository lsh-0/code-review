// the in-page find feature: an overlay bar that searches the whole page,
// highlights every match, and steps a selection through them. WebKitGTK does not
// wire up the browser's native Ctrl-f, so this replaces it and stays as close to
// native find as possible — the entire application content is searched, as a
// browser's own find would. The match-locating and index-stepping decisions are
// pure (see `core/find.ts`); this module owns the DOM — the bar, the `<mark>`
// highlights, and scrolling the active match into view.

import { byId, el } from "../dom.ts";
import {
  type FindMatch,
  findMatches,
  nextMatchIndex,
  prevMatchIndex,
} from "../core/find.ts";

// the id of the find bar, so it is found/toggled without re-querying by class.
const findBarId = "find-bar";

// the id of the search root: the whole application content. The find bar and the
// modals sit outside `#app`, so searching it covers the visible page (diff, file
// list, header) without matching the find bar's own counter.
const searchRootId = "app";

// the class marking a highlighted match span, and the modifier on the active one.
const matchClass = "find-match";
const activeMatchClass = "find-match-active";

// elements carrying this class are hidden (e.g. the collapsed change banner);
// their text is skipped so a hidden node cannot contribute phantom matches.
const hiddenClass = "hidden";

// the in-page-find runtime state: the open bar's input/count nodes, the live
// match spans, and which one is active. Held module-private so the keyboard
// wiring in `main.ts` drives it through `openFind`/`closeFind` without threading
// the controller around.
interface FindState {
  bar: HTMLElement;
  input: HTMLInputElement;
  countLabel: HTMLElement;
  marks: HTMLElement[];
  activeIndex: number;
}

let active_find: FindState | null = null;

// open the find bar (creating it on first use), focus and select its input, and
// re-run the current term so reopening over an existing search restores the
// highlights. No-op'd into a focus if already open.
export function openFind(): void {
  // an existing bar still in the live document just re-focuses; a tracked bar
  // that is no longer in the document (e.g. the document was replaced) is stale,
  // so fall through and rebuild.
  if (active_find && byId(findBarId)) {
    focusInput(active_find.input, true);
    return;
  }
  active_find = null;
  const root = byId(searchRootId);
  if (!root) {
    return;
  }
  active_find = buildFindBar();
  runSearch();
  focusInput(active_find.input, false);
}

// focus the find input (and optionally select its text), guarding the methods
// the deno-dom test shim does not implement so the find flow runs unbroken under
// test as it does under WebKitGTK.
function focusInput(input: HTMLInputElement, select: boolean): void {
  input.focus?.();
  if (select) {
    input.select?.();
  }
}

// close the find bar: clear every highlight, remove the bar, and return focus to
// the diff so the next keypress scrolls the page. No-op when not open.
export function closeFind(): void {
  if (!active_find) {
    return;
  }
  clearHighlights();
  active_find.bar.remove();
  active_find = null;
  // guarded: the deno-dom test shim does not implement focus.
  byId("diff-view")?.focus?.();
}

// whether the find bar is currently open, for the keyboard wiring to decide
// whether Escape should close it.
export function findIsOpen(): boolean {
  return active_find !== null;
}

// step the active match forward / backward, for the global F3 / Shift+F3 wiring
// in `main.ts` (the bar's own input binds Enter / Shift+Enter to the same steps).
// A no-op when the bar is closed or there are no matches.
export function findNext(): void {
  step(1);
}

export function findPrev(): void {
  step(-1);
}

// build the find bar DOM (input, match counter, prev/next/close buttons), wire
// its handlers, and mount it. Returns the fresh find state.
function buildFindBar(): FindState {
  const input = el("input", {
    attrs: {
      type: "text",
      id: "find-input",
      placeholder: "Find in page…",
      autocomplete: "off",
      spellcheck: "false",
      "aria-label": "Find in page",
    },
  }) as HTMLInputElement;

  const countLabel = el("span", {
    classes: ["find-count"],
    attrs: { id: "find-count" },
    text: "",
  });

  const prevBtn = el("button", {
    classes: ["find-nav"],
    text: "‹",
    attrs: {
      title: "Previous match (Shift+Enter)",
      "aria-label": "Previous match",
    },
    onClick: () => step(-1),
  });
  const nextBtn = el("button", {
    classes: ["find-nav"],
    text: "›",
    attrs: { title: "Next match (Enter)", "aria-label": "Next match" },
    onClick: () => step(1),
  });
  const closeBtn = el("button", {
    classes: ["find-close"],
    text: "×",
    attrs: { title: "Close (Esc)", "aria-label": "Close find" },
    onClick: () => closeFind(),
  });

  const bar = el("div", {
    attrs: { id: findBarId, role: "search" },
    children: [input, countLabel, prevBtn, nextBtn, closeBtn],
  });

  input.addEventListener("input", () => runSearch());
  input.addEventListener("keydown", (ev) => {
    const key = (ev as KeyboardEvent).key;
    if (key === "Enter") {
      ev.preventDefault();
      step((ev as KeyboardEvent).shiftKey ? -1 : 1);
    } else if (key === "Escape" || key === "Esc") {
      ev.preventDefault();
      closeFind();
    }
  });

  document.body.appendChild(bar);
  return { bar, input, countLabel, marks: [], activeIndex: -1 };
}

// re-run the search for the input's current term: clear the old highlights,
// re-highlight, select the first match (if any), and update the counter.
function runSearch(): void {
  if (!active_find) {
    return;
  }
  clearHighlights();
  const term = findTerm();
  active_find.marks = highlightMatches(term);
  active_find.activeIndex = active_find.marks.length > 0 ? 0 : -1;
  applyActive();
  updateCount();
}

// the find input's current term. `?? ""` guards the deno-dom test shim, whose
// freshly-created input reports an undefined `value` where a browser reports "".
function findTerm(): string {
  return active_find?.input.value ?? "";
}

// move the active selection by `delta` (+1 next, -1 previous) with wraparound,
// then scroll it into view. No-op when there are no matches.
function step(delta: number): void {
  if (!active_find || active_find.marks.length === 0) {
    return;
  }
  const count = active_find.marks.length;
  active_find.activeIndex = delta > 0
    ? nextMatchIndex(active_find.activeIndex, count)
    : prevMatchIndex(active_find.activeIndex, count);
  applyActive();
  updateCount();
}

// reflect `activeIndex` onto the marks: clear the active class everywhere, set it
// on the current mark, and scroll that mark into view.
function applyActive(): void {
  if (!active_find) {
    return;
  }
  for (const mark of active_find.marks) {
    mark.classList.remove(activeMatchClass);
  }
  const current = active_find.marks[active_find.activeIndex];
  if (current) {
    current.classList.add(activeMatchClass);
    // guarded: the deno-dom test shim does not implement scrollIntoView.
    current.scrollIntoView?.({ block: "center", inline: "nearest" });
  }
}

// update the "n of m" counter (or "No matches" for a non-empty term that found
// nothing, and empty for an empty term).
function updateCount(): void {
  if (!active_find) {
    return;
  }
  const total = active_find.marks.length;
  if (findTerm().trim() === "") {
    active_find.countLabel.textContent = "";
  } else if (total === 0) {
    active_find.countLabel.textContent = "No matches";
  } else {
    active_find.countLabel.textContent = `${
      active_find.activeIndex + 1
    } of ${total}`;
  }
}

// highlight every occurrence of `term` across the whole page's text, returning
// the inserted mark spans in document order. Each text node is searched
// independently and its matched substrings are wrapped in `<mark>`, so the
// highlights compose with the existing syntax-highlight spans (a match within one
// token highlights; a match straddling a token boundary is not joined — rare per
// line and acceptable). Returns an empty list for an empty term or no root.
function highlightMatches(term: string): HTMLElement[] {
  const root = byId(searchRootId);
  if (!root || term.trim() === "") {
    return [];
  }
  const marks: HTMLElement[] = [];
  // collect the text nodes first: highlighting mutates the tree, so walking it
  // live would visit the marks we insert.
  for (const textNode of collectTextNodes(root)) {
    highlightTextNode(textNode, term, marks);
  }
  return marks;
}

// the text nodes under `root`, skipping those already inside a find mark (so a
// re-run that failed to clear cannot nest marks), those inside a hidden subtree
// (a browser's find does not match hidden text), and empty/whitespace-only nodes
// (nothing to match). Gathered into an array so the caller can mutate the tree
// without disturbing the walk.
function collectTextNodes(root: Element): Text[] {
  const nodes: Text[] = [];
  const walk = (node: Node): void => {
    for (const child of Array.from(node.childNodes)) {
      if (child.nodeType === 3) {
        const text = child as Text;
        if ((text.textContent ?? "").trim() !== "") {
          nodes.push(text);
        }
      } else if (child.nodeType === 1) {
        const elem = child as Element;
        if (
          !elem.classList.contains(matchClass) &&
          !elem.classList.contains(hiddenClass)
        ) {
          walk(elem);
        }
      }
    }
  };
  walk(root);
  return nodes;
}

// replace `textNode` with a sequence of plain text nodes and `<mark>` spans, one
// mark per match of `term` in the node's text. The inserted marks are appended to
// `marks` in order. A node with no match is left untouched.
function highlightTextNode(
  textNode: Text,
  term: string,
  marks: HTMLElement[],
): void {
  const text = textNode.textContent ?? "";
  const found: FindMatch[] = findMatches(text, term);
  if (found.length === 0) {
    return;
  }
  const parent = textNode.parentNode;
  if (!parent) {
    return;
  }

  const frag = document.createDocumentFragment();
  let cursor = 0;
  for (const m of found) {
    if (m.start > cursor) {
      frag.appendChild(document.createTextNode(text.slice(cursor, m.start)));
    }
    const mark = el("mark", {
      classes: [matchClass],
      text: text.slice(m.start, m.start + m.length),
    });
    frag.appendChild(mark);
    marks.push(mark);
    cursor = m.start + m.length;
  }
  if (cursor < text.length) {
    frag.appendChild(document.createTextNode(text.slice(cursor)));
  }
  parent.replaceChild(frag, textNode);
}

// remove every find highlight, restoring the original text: each mark is replaced
// by a plain text node of its content, and the touched parents have their split
// text nodes re-merged so the next search sees whole runs again (and the page DOM
// is left as it was).
function clearHighlights(): void {
  if (!active_find) {
    return;
  }
  const parents = new Set<Node>();
  for (const mark of active_find.marks) {
    const parent = mark.parentNode;
    if (!parent) {
      continue;
    }
    parent.replaceChild(
      document.createTextNode(mark.textContent ?? ""),
      mark,
    );
    parents.add(parent);
  }
  for (const parent of parents) {
    mergeAdjacentTextNodes(parent);
  }
  active_find.marks = [];
  active_find.activeIndex = -1;
}

// merge each run of adjacent text-node children of `parent` into a single text
// node. Replacing marks with text nodes leaves the surrounding text split (a mark
// can sit between two text fragments); re-merging keeps a later search from
// missing a term that straddles the seam. This stands in for the DOM's
// `normalize()`, which the deno-dom test shim does not implement.
function mergeAdjacentTextNodes(parent: Node): void {
  let child = parent.firstChild;
  while (child) {
    if (child.nodeType !== 3) {
      child = child.nextSibling;
      continue;
    }
    let next = child.nextSibling;
    while (next && next.nodeType === 3) {
      child.textContent = (child.textContent ?? "") + (next.textContent ?? "");
      const after = next.nextSibling;
      parent.removeChild(next);
      next = after;
    }
    child = next;
  }
}
