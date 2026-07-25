// DOM integration tests for the in-page find render layer: render text into the
// page, drive the find bar, and assert on the produced tree — the mark spans
// inserted, the active selection, the match counter, whole-page scope, and that
// closing restores the original text. Run with --allow-read --allow-net (deno-dom
// WASM init). These exercise the DOM splicing the pure `core/find.ts` tests
// cannot.

import { assert, assertEquals } from "@std/assert";
import { setupDom } from "./dom_test_setup.ts";
import { byId } from "../dom.ts";
import { closeFind, findIsOpen, findNext, findPrev, openFind } from "./find.ts";

// seed the diff surface with two lines whose content carries known text, mirroring
// the `.line-content` nodes the real diff renders. Returns the surface element.
function seedDiff(): HTMLElement {
  const surface = byId("diff-content");
  if (!surface) {
    throw new Error("test shell missing #diff-content");
  }
  surface.innerHTML = `<div class="line-content">the quick brown Fox</div>` +
    `<div class="line-content">a fox and a foal</div>`;
  return surface;
}

// type a term into the open find bar's input and fire its `input` handler, the
// way a keystroke would.
function typeFind(term: string): void {
  const input = byId<HTMLInputElement>("find-input");
  if (!input) {
    throw new Error("find input not present");
  }
  input.value = term;
  input.dispatchEvent(new Event("input"));
}

// the find marks currently in the surface, in document order.
function marks(): HTMLElement[] {
  return Array.from(
    document.querySelectorAll<HTMLElement>(".find-match"),
  );
}

Deno.test("find: opening then searching wraps each case-insensitive match in a mark", () => {
  setupDom();
  seedDiff();

  openFind();
  typeFind("fox");

  const found = marks();
  assertEquals(found.length, 2);
  assertEquals(found.map((m) => m.textContent), ["Fox", "fox"]);
  // the matched text keeps its original case, the surrounding text is preserved.
  assertEquals(
    byId("diff-content")?.textContent,
    "the quick brown Foxa fox and a foal",
  );

  closeFind();
});

Deno.test("find: the first match is active and the counter reads its position", () => {
  setupDom();
  seedDiff();

  openFind();
  typeFind("fo");

  // "fo" matches Fox, fox, foal → three.
  const found = marks();
  assertEquals(found.length, 3);
  assert(found[0].classList.contains("find-match-active"));
  assertEquals(byId("find-count")?.textContent, "1 of 3");
});

Deno.test("find: a non-matching term reports no matches and inserts no marks", () => {
  setupDom();
  seedDiff();

  openFind();
  typeFind("zebra");

  assertEquals(marks().length, 0);
  assertEquals(byId("find-count")?.textContent, "No matches");
});

Deno.test("find: closing removes every mark and restores the original text", () => {
  setupDom();
  const surface = seedDiff();
  const before = surface.textContent;

  openFind();
  typeFind("fox");
  assertEquals(marks().length, 2);

  closeFind();

  assertEquals(findIsOpen(), false);
  assertEquals(marks().length, 0);
  assertEquals(byId("find-bar"), null);
  assertEquals(surface.textContent, before);
});

Deno.test("find: clearing the term removes the highlights and empties the counter", () => {
  setupDom();
  seedDiff();

  openFind();
  typeFind("fox");
  assertEquals(marks().length, 2);

  typeFind("");
  assertEquals(marks().length, 0);
  assertEquals(byId("find-count")?.textContent, "");

  closeFind();
});

Deno.test("find: searches the whole page, including the file list", () => {
  setupDom();
  seedDiff();
  const files = byId("files");
  if (!files) {
    throw new Error("test shell missing #files");
  }
  files.innerHTML = `<div class="file-item">src/fox.ts</div>`;

  openFind();
  typeFind("fox");

  // two in the diff ("Fox", "fox") plus one in the file list = three.
  assertEquals(marks().length, 3);
  assert(
    files.querySelector(".find-match") !== null,
    "the file-list occurrence should be highlighted",
  );

  closeFind();
});

Deno.test("find: hidden subtrees are not searched", () => {
  setupDom();
  seedDiff();
  const banner = byId("review-changed-banner");
  if (!banner) {
    throw new Error("test shell missing #review-changed-banner");
  }
  // the banner carries the `hidden` class in the shell; its text must not count.
  banner.textContent = "a fox in a hidden banner";

  openFind();
  typeFind("fox");

  // still only the two diff occurrences; the hidden banner's is skipped.
  assertEquals(marks().length, 2);

  closeFind();
});

Deno.test("find: findNext / findPrev step the active match with wraparound", () => {
  setupDom();
  seedDiff();

  openFind();
  // "fo" matches Fox, fox, foal → three, the first active.
  typeFind("fo");
  assertEquals(byId("find-count")?.textContent, "1 of 3");

  findNext();
  assertEquals(byId("find-count")?.textContent, "2 of 3");
  findNext();
  assertEquals(byId("find-count")?.textContent, "3 of 3");
  findNext(); // wraps to the first
  assertEquals(byId("find-count")?.textContent, "1 of 3");
  findPrev(); // wraps back to the last
  assertEquals(byId("find-count")?.textContent, "3 of 3");

  closeFind();
});

Deno.test("find: stepping is a no-op when the bar is closed", () => {
  setupDom();
  seedDiff();

  // must not throw and must not open the bar.
  findNext();
  findPrev();
  assertEquals(findIsOpen(), false);
});

Deno.test("find: re-running over different terms does not nest or leak marks", () => {
  setupDom();
  const surface = seedDiff();
  const before = surface.textContent;

  openFind();
  typeFind("fox");
  typeFind("a");
  typeFind("foal");

  // only the final term's matches remain, and the text is intact between runs.
  assertEquals(marks().map((m) => m.textContent), ["foal"]);
  // restore for the next assertion: closing must still return the original text.
  closeFind();
  assertEquals(surface.textContent, before);
});
