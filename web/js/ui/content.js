// Empty states and the markdown-lite renderer for agent replies.
import { html, t, useMemo } from "../core.js";
import { Icon } from "../icons.js";

export function Empty({ icon = "sparkles", title, children }) {
  return html`<div class="empty"><${Icon} name=${icon} size=${30} /><h3>${title}</h3><div class="small" style="margin-top:6px">${children}</div></div>`;
}

// Markdown-lite renders agent replies as elements (never innerHTML), covering the
// constructs agents actually use: paragraphs, lists, fenced code, `code`, **bold**, headings.
// Parsing runs again only when the text changes, not on every re-render of the timeline.
export function Markdown({ text }) {
  const blocks = useMemo(() => parse(text), [text]);
  return html`<div class="md">${blocks}</div>`;
}

function parse(text) {
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
  return blocks;
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
