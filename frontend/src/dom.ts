/** Small helpers for building DOM nodes without a framework. */

type EventMap = {
  [K in keyof HTMLElementEventMap]?: (event: HTMLElementEventMap[K]) => void;
};

export interface ElementOptions {
  className?: string;
  text?: string;
  html?: string;
  title?: string;
  attrs?: Record<string, string | number | boolean | null>;
  dataset?: Record<string, string>;
  on?: EventMap;
}

/** el creates an element, applies options and appends children. */
export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  options: ElementOptions = {},
  children: (Node | string | null | undefined | false)[] = [],
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);

  if (options.className) node.className = options.className;
  if (options.text !== undefined) node.textContent = options.text;
  if (options.html !== undefined) node.innerHTML = options.html;
  if (options.title) node.title = options.title;

  for (const [name, value] of Object.entries(options.attrs ?? {})) {
    if (value === null || value === false) continue;
    node.setAttribute(name, value === true ? "" : String(value));
  }
  for (const [name, value] of Object.entries(options.dataset ?? {})) {
    node.dataset[name] = value;
  }
  for (const [name, handler] of Object.entries(options.on ?? {})) {
    node.addEventListener(name, handler as EventListener);
  }

  append(node, children);
  return node;
}

/** append adds children, skipping empty values so conditionals read cleanly. */
export function append(parent: Node, children: (Node | string | null | undefined | false)[]): void {
  for (const child of children) {
    if (child === null || child === undefined || child === false) continue;
    parent.appendChild(typeof child === "string" ? document.createTextNode(child) : child);
  }
}

/** clear removes every child of a node. */
export function clear(node: Node): void {
  while (node.firstChild) node.removeChild(node.firstChild);
}

/** replace swaps the contents of a node for new children. */
export function replace(node: Node, children: (Node | string | null | undefined | false)[]): void {
  clear(node);
  append(node, children);
}

/** show toggles visibility without losing the node's place in the layout flow. */
export function show(node: HTMLElement, visible: boolean): void {
  node.hidden = !visible;
}

/** icon renders one of the small inline glyphs used across the interface. */
export function icon(name: "check" | "warn" | "cross" | "chevron" | "folder" | "gear" | "refresh"): HTMLElement {
  const glyphs: Record<string, string> = {
    check: "✓",
    warn: "⚠",
    cross: "✕",
    chevron: "›",
    folder: "🗀",
    gear: "⚙",
    refresh: "↻",
  };
  return el("span", { className: "icon", text: glyphs[name], attrs: { "aria-hidden": "true" } });
}
