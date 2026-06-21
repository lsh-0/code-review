// small DOM-construction helpers shared by the render layer. Gathering element
// assembly here keeps the render modules reading as structure rather than
// imperative DOM calls. These touch the DOM and so live in the render layer, not
// the pure core.

// options for building an element: css classes, text content, attributes, and a
// click handler — the combination almost every node in the UI needs.
export interface ElOptions {
  classes?: string[];
  text?: string;
  attrs?: Record<string, string>;
  onClick?: (ev: Event) => void;
  children?: Node[];
}

// create an element with the given tag and options. Returns the typed element
// so callers keep `HTMLInputElement`-style access without casting.
export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  opts: ElOptions = {},
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (opts.classes) {
    node.classList.add(...opts.classes);
  }
  if (opts.text !== undefined) {
    node.textContent = opts.text;
  }
  if (opts.attrs) {
    for (const [k, v] of Object.entries(opts.attrs)) {
      node.setAttribute(k, v);
    }
  }
  if (opts.onClick) {
    node.addEventListener("click", opts.onClick);
  }
  if (opts.children) {
    for (const child of opts.children) {
      node.appendChild(child);
    }
  }
  return node;
}

// look up an element by id, returning null when absent. The render layer guards
// for null where the bridge or DOM may not be ready.
export function byId<T extends HTMLElement = HTMLElement>(
  id: string,
): T | null {
  return document.getElementById(id) as T | null;
}

// require an element by id, throwing when absent. Used for the structural ids
// that `index.html` always provides (the file list, diff content, modals); a
// missing one is a programming error, not a runtime condition to degrade past.
export function requireId<T extends HTMLElement = HTMLElement>(id: string): T {
  const node = document.getElementById(id);
  if (!node) {
    throw new Error(`required element #${id} not found`);
  }
  return node as T;
}

// clear an element's children. Uses a removal loop rather than
// `replaceChildren()` so it works identically under WebKitGTK and the deno-dom
// test shim (which does not implement `replaceChildren`).
export function clear(node: Element): void {
  while (node.firstChild) {
    node.removeChild(node.firstChild);
  }
}

// escape text for safe insertion as HTML. Used for the syntax-highlighting
// plain-text fallback: when no highlighter result is available the raw code
// must be escaped before it goes in via innerHTML, since that node is otherwise
// fed only the highlighter's already-escaped output.
//
// The replace-chain is deliberate, not a shortcut. It is the standard way to
// escape a *string*: the browser-native alternative (set `el.textContent`, read
// back `el.innerHTML`) needs a DOM element, so it cannot run in the pure
// highlight path, and it behaves inconsistently under the deno-dom test shim
// (the same reason `clear()` avoids `replaceChildren`). Everywhere a DOM node is
// in hand, `textContent` is used instead and auto-escapes; this function is the
// string-only escape for the one path (the recognised-language branch in
// `render/diff.ts`) that must inject live HTML via innerHTML.
export function escapeHTML(text: string): string {
  return text
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
