// How one timeline event looks: agent text, thinking, tool calls, logs, results.
import { html, lang, t, useState } from "../core.js";
import { Icon } from "../icons.js";
import { Markdown } from "../ui.js";

export function Ev({ e }) {
  const label = (icon, name, extra) => html`<div class="ev-label"><b>${name}</b>${extra}<span class="spacer"></span>${e.ts && new Date(e.ts).toLocaleTimeString(lang === "en" ? "en-GB" : "vi-VN")}</div>`;
  switch (e.kind) {
    case "text":
      return html`<div class="ev text"><div class="ev-icon"><${Icon} name="sparkles" size=${14} /></div><div class="ev-body">
        ${label("sparkles", "Agent", e.title && html`<span class="pill">${e.title}</span>`)}<div class="bubble"><${Markdown} text=${e.body} /></div></div></div>`;
    case "thinking":
      return html`<${Thinking} e=${e} />`;
    case "tool_call": {
      const r = e.result;
      return html`<div class=${"ev tool" + (r?.error ? " err" : "")}><div class="ev-icon"><${Icon} name="wrench" size=${14} /></div><div class="ev-body">
        ${label("wrench", e.tool || "tool", html`${e.title && html`<span class="ellipsis" style="max-width:60%">${e.title}</span>`}${!r ? html`<span class="pill run"><span class="dot"></span>${t("đang chạy")}</span>` : r.error ? html`<span class="pill bad">${t("lỗi")}</span>` : null}`)}
        <div class="tool-card">${e.body && html`<div class="tool-cmd">${e.body}</div>`}
          ${r && html`<details class=${"tool-out" + (r.error ? " err" : "")} open=${r.error}>
            <summary><${Icon} name="chevron" size=${12} />${r.body ? t("Kết quả · {n} dòng", { n: r.body.split("\n").length }) : t("Không có đầu ra")}</summary>
            ${r.body && html`<pre>${r.body}</pre>`}</details>`}
        </div></div></div>`;
    }
    case "tool_result":
      return html`<div class="ev tool"><div class="ev-icon"><${Icon} name="terminal" size=${14} /></div><div class="ev-body">${label("", t("Kết quả"))}<pre class="logs">${e.body}</pre></div></div>`;
    case "logs":
      return html`<div class="ev"><div class="ev-icon"><${Icon} name="terminal" size=${14} /></div><div class="ev-body">${label("", t("Nhật ký"))}
        <div class="logs">${e.lines.map((l) => html`<div class=${l.kind === "stderr" ? "se" : ""}>${l.body}</div>`)}</div></div></div>`;
    case "result":
      return html`<div class=${"ev result" + (e.error ? " err" : "")}><div class="ev-icon"><${Icon} name=${e.error ? "alert" : "check"} size=${14} /></div><div class="ev-body">
        ${label("", e.error ? t("Kết thúc có lỗi") : t("Kết quả"))}<div class=${"result-card" + (e.error ? " bad" : "")}>
        ${e.title && html`<div class="small" style="font-weight:650;margin-bottom:4px">${e.title}</div>`}${e.body && html`<${Markdown} text=${e.body} />`}</div></div></div>`;
    case "error":
      return html`<div class="ev err"><div class="ev-icon"><${Icon} name="alert" size=${14} /></div><div class="ev-body">${label("", t("Lỗi"))}<pre class="logs" style="color:#fca5a5">${e.body}</pre></div></div>`;
    case "status":
      return html`<div class="status-line"><span title=${e.body}>${e.title}${e.body && html`<span class="mono" style="font-size:11px"> · ${e.body}</span>`}</span></div>`;
    case "system":
      return html`<div class="ev"><div class="ev-icon"><${Icon} name="cpu" size=${14} /></div><div class="ev-body">${label("", e.title || t("Hệ thống"))}
        <div class="small muted mono" style="white-space:pre-wrap">${e.body}</div></div></div>`;
  }
  return null;
}

function Thinking({ e }) {
  const [open, setOpen] = useState(false);
  return html`<div class="ev"><div class="ev-icon"><${Icon} name="brain" size=${14} /></div><div class="ev-body">
    <div class="ev-label"><b>${t("Suy nghĩ")}</b><span class="muted">${open ? t("bấm để thu gọn") : t("bấm để mở")}</span></div>
    <div class=${"thinking" + (open ? " open" : "")} onClick=${() => setOpen(!open)}>${e.body}</div></div></div>`;
}
