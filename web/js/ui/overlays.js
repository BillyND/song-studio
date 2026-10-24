// Things drawn above the page: portals, modals, the image lightbox, toasts, the column resize handle.
import { html, localStorageSet, render, t, toast, useEffect, useLayoutEffect, useRef, useStore } from "../core.js";
import { Icon } from "../icons.js";
import { Btn } from "./controls.js";

// Portal renders overlays straight into <body>: a fixed element inside an animated
// (transformed) page would otherwise be positioned against the page, not the window.
export function Portal({ children }) {
  const host = useRef(null);
  if (!host.current) host.current = document.createElement("div");
  useLayoutEffect(() => {
    document.body.appendChild(host.current);
    return () => { render(null, host.current); host.current.remove(); };
  }, []);
  useLayoutEffect(() => { render(children, host.current); });
  return null;
}

export function Modal({ title, onClose, children, footer }) {
  useEffect(() => {
    const k = (e) => e.key === "Escape" && onClose();
    addEventListener("keydown", k);
    return () => removeEventListener("keydown", k);
  }, []);
  return html`<${Portal}><div class="modal-back" onClick=${(e) => e.target === e.currentTarget && onClose()}>
    <div class="modal" role="dialog" aria-modal="true" aria-label=${title}>
      <div class="card-head modal-head"><h2>${title}</h2><span class="spacer"></span><${Btn} kind="ghost" size="sm" icon="x" title=${t("Đóng")} onClick=${onClose} /></div>
      <div class="card-body modal-body">${children}</div>
      ${footer && html`<div class="card-head modal-foot">${footer}</div>`}
    </div></div></${Portal}>`;
}

export function Lightbox({ src, caption, onClose }) {
  useEffect(() => {
    const k = (e) => e.key === "Escape" && onClose();
    addEventListener("keydown", k);
    return () => removeEventListener("keydown", k);
  }, []);
  return html`<${Portal}><div class="lightbox" onClick=${onClose}><img src=${src} alt=${caption || ""} />${caption && html`<div class="cap">${caption}</div>`}</div></${Portal}>`;
}

export function Toasts() {
  const toasts = useStore((s) => s.toasts);
  return html`<div class="toasts" aria-live="polite">${toasts.map((t) => html`
    <div class=${"toast " + t.kind}><${Icon} name=${t.kind === "bad" ? "alert" : "check"} /><span>${t.message}</span></div>`)}</div>`;
}

// ResizeHandle drags the left edge of the right column (run console or agent chat) to set its width.
export function ResizeHandle() {
  const down = (e) => {
    e.preventDefault();
    const root = document.documentElement;
    const move = (ev) => root.style.setProperty("--drawer", Math.round(Math.max(360, Math.min(innerWidth - 520, innerWidth - ev.clientX))) + "px");
    const up = () => {
      removeEventListener("pointermove", move);
      removeEventListener("pointerup", up);
      root.classList.remove("resizing");
      localStorageSet("drawerW", root.style.getPropertyValue("--drawer"));
    };
    root.classList.add("resizing");
    addEventListener("pointermove", move);
    addEventListener("pointerup", up);
  };
  return html`<div class="resize-handle" onPointerDown=${down} title=${t("Kéo để đổi độ rộng")}></div>`;
}
