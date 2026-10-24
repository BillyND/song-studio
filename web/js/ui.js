// Shared UI pieces used by every view.
import { html, render, useState, useEffect, useRef, useLayoutEffect, store, useStore } from "./core.js";
import { Icon } from "./icons.js";

export const STATUS = {
  queued: { label: "Đang chờ", cls: "" },
  running: { label: "Đang chạy", cls: "run" },
  succeeded: { label: "Hoàn thành", cls: "ok" },
  failed: { label: "Thất bại", cls: "bad" },
  stopped: { label: "Đã dừng", cls: "warn" },
  interrupted: { label: "Gián đoạn", cls: "warn" },
  done: { label: "Xong", cls: "ok" },
  todo: { label: "Chưa làm", cls: "" },
  stale: { label: "Cần làm lại", cls: "warn" },
  skip: { label: "Bỏ qua", cls: "ok" },
};

export function StatusPill({ status, label }) {
  const s = STATUS[status] || { label: status, cls: "" };
  return html`<span class=${"pill " + s.cls}><span class="dot"></span>${label || s.label}</span>`;
}

export function Btn({ icon, children, kind = "", size = "", onClick, disabled, title, type = "button", busy }) {
  const cls = ["btn", kind, size, !children ? "icon" : ""].filter(Boolean).join(" ");
  return html`<button type=${type} class=${cls} onClick=${onClick} disabled=${disabled || busy} title=${title} aria-label=${title}>
    ${busy ? html`<${Icon} name="loader" class="spin" />` : icon ? html`<${Icon} name=${icon} />` : null}${children}
  </button>`;
}

export function Switch({ on, onChange, children }) {
  return html`<span class=${"switch" + (on ? " on" : "")} role="switch" aria-checked=${on} tabindex="0"
    onClick=${() => onChange(!on)} onKeyDown=${(e) => (e.key === " " || e.key === "Enter") && (e.preventDefault(), onChange(!on))}><i></i>${children}</span>`;
}

export function Seg({ value, options, onChange }) {
  return html`<div class="seg" role="radiogroup">${options.map((o) => html`
    <button type="button" class=${o.value === value ? "on" : ""} role="radio" aria-checked=${o.value === value} onClick=${() => onChange(o.value)}>
      ${o.icon && html`<${Icon} name=${o.icon} size=${13} />`}${o.label}</button>`)}</div>`;
}

export function AgentIcon({ agent }) {
  const letter = { claude: "C", omp: "O", codex: "X", script: "›_" }[agent] || "?";
  return html`<span class=${"agent-ico " + (agent || "script")} title=${agent || "script"}>${letter}</span>`;
}

// AgentPicker lets each agent action choose its CLI and optional model.
export function AgentPicker({ value, onChange, model, onModel }) {
  const boot = useStore((s) => s.boot);
  const agents = boot?.agents || [];
  return html`<span class="agent-pick">
    <${Seg} value=${value} onChange=${onChange} options=${agents.map((a) => ({ value: a.name, label: a.name + (a.available ? "" : " (thiếu)") }))} />
    ${onModel && html`<input class="input" style="width:150px;height:34px" placeholder="model (mặc định)" value=${model} onInput=${(e) => onModel(e.target.value)} />`}
  </span>`;
}

export function useAgentChoice() {
  const def = useStore((s) => s.boot?.config?.defaultAgent || "claude");
  const [agent, setAgent] = useState(def);
  const [model, setModel] = useState("");
  useEffect(() => setAgent(def), [def]);
  return { agent, setAgent, model, setModel };
}

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
      <div class="card-head modal-head"><h2>${title}</h2><span class="spacer"></span><${Btn} kind="ghost" size="sm" icon="x" title="Đóng" onClick=${onClose} /></div>
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

export function Empty({ icon = "sparkles", title, children }) {
  return html`<div class="empty"><${Icon} name=${icon} size=${30} /><h3>${title}</h3><div class="small" style="margin-top:6px">${children}</div></div>`;
}

// Markdown-lite renders agent replies as elements (never innerHTML), covering the
// constructs agents actually use: paragraphs, lists, fenced code, `code`, **bold**, headings.
export function Markdown({ text }) {
  const blocks = [];
  const lines = String(text || "").split("\n");
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (line.startsWith("```")) {
      const buf = [];
      for (i++; i < lines.length && !lines[i].startsWith("```"); i++) buf.push(lines[i]);
      blocks.push(html`<pre>${buf.join("\n")}</pre>`);
    } else if (/^\s*[-*] /.test(line)) {
      const items = [];
      for (; i < lines.length && /^\s*[-*] /.test(lines[i]); i++) items.push(lines[i].replace(/^\s*[-*] /, ""));
      i--;
      blocks.push(html`<ul>${items.map((it) => html`<li>${inline(it)}</li>`)}</ul>`);
    } else if (/^\s*\d+\. /.test(line)) {
      const items = [];
      for (; i < lines.length && /^\s*\d+\. /.test(lines[i]); i++) items.push(lines[i].replace(/^\s*\d+\. /, ""));
      i--;
      blocks.push(html`<ol>${items.map((it) => html`<li>${inline(it)}</li>`)}</ol>`);
    } else if (/^#{1,4} /.test(line)) {
      blocks.push(html`<p><strong>${inline(line.replace(/^#+ /, ""))}</strong></p>`);
    } else if (line.trim() === "") {
      continue;
    } else if (line.startsWith("|")) {
      const rows = [];
      for (; i < lines.length && lines[i].startsWith("|"); i++) if (!/^\|[\s:|-]+\|$/.test(lines[i])) rows.push(lines[i]);
      i--;
      blocks.push(html`<pre>${rows.join("\n")}</pre>`);
    } else {
      const para = [line];
      while (i + 1 < lines.length && lines[i + 1].trim() && !/^(```|\s*[-*] |\s*\d+\. |#{1,4} |\|)/.test(lines[i + 1])) para.push(lines[++i]);
      blocks.push(html`<p>${inline(para.join(" "))}</p>`);
    }
  }
  return html`<div class="md">${blocks}</div>`;
}

function inline(s) {
  const out = [];
  const re = /(`[^`]+`|\*\*[^*]+\*\*)/g;
  let last = 0, m;
  while ((m = re.exec(s))) {
    if (m.index > last) out.push(s.slice(last, m.index));
    const t = m[0];
    out.push(t.startsWith("`") ? html`<code>${t.slice(1, -1)}</code>` : html`<strong>${t.slice(2, -2)}</strong>`);
    last = m.index + t.length;
  }
  if (last < s.length) out.push(s.slice(last));
  return out;
}

// PresetChips offers the saved prompt snippets (Cài đặt → Prompt mở rộng) as one-click inserts.
export function PresetChips({ value, onChange }) {
  const presets = useStore((s) => s.boot?.config?.presets || []);
  if (!presets.length) return null;
  const add = (t) => {
    if ((value || "").includes(t)) return;
    onChange(((value || "").trim() ? value.trim() + "\n" : "") + t);
  };
  return html`<div class="preset-row">${presets.map((p) => html`
    <button type="button" class=${"preset" + ((value || "").includes(p.text) ? " on" : "")} title=${p.text} onClick=${() => add(p.text)}>
      <${Icon} name=${(value || "").includes(p.text) ? "check" : "plus"} size=${12} />${p.label}</button>`)}</div>`;
}

export function Field({ label, children, hint }) {
  return html`<label class="field">${label}${children}${hint && html`<span class="muted small" style="font-weight:400">${hint}</span>`}</label>`;
}
