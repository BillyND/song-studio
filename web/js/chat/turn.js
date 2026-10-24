// One turn: what the user wrote and the agent's reply, with tool calls folded into one line.
import { Ev } from "../console/events.js";
import { groupEvents, useRunStream } from "../console/stream.js";
import { ago, html, isActive, openRun, runDuration, t, useMemo, useState } from "../core.js";
import { Icon } from "../icons.js";
import { StatusPill } from "../ui.js";

// What a chat shows of a run: the conversation, not the bookkeeping around it.
const shown = (e) => e.kind !== "system" && (e.kind !== "status" || e.error) && !(e.kind === "result" && !e.body && !e.error);

// Tool calls and logs in a row fold into one line; the details open on click.
function foldTools(items) {
  const out = [];
  for (const e of items) {
    if (e.kind === "tool_call" || e.kind === "tool_result" || e.kind === "logs") {
      const prev = out[out.length - 1];
      if (prev && prev.kind === "tools") { prev.items.push(e); continue; }
      out.push({ kind: "tools", seq: e.seq, items: [e] });
      continue;
    }
    out.push(e);
  }
  return out;
}

function ToolsFold({ items, live }) {
  const [open, setOpen] = useState(false);
  const calls = items.filter((e) => e.kind === "tool_call");
  const running = live && calls.some((e) => !e.result);
  const failed = calls.some((e) => e.result?.error);
  const names = [...new Set(calls.map((e) => e.tool || "tool"))].join(", ");
  const last = calls[calls.length - 1];
  return html`<div class=${"tools-fold" + (open ? " open" : "")}>
    <button type="button" class="tools-sum" aria-expanded=${open} onClick=${() => setOpen(!open)}>
      <${Icon} name=${running ? "loader" : "wrench"} size=${13} class=${running ? "spin" : ""} />
      <b>${calls.length === 1 ? t("1 thao tác") : calls.length ? t("{n} thao tác", { n: calls.length }) : t("Nhật ký")}</b>
      <span class="muted ellipsis">${running && last ? last.title || last.tool : names}</span>
      ${failed && html`<span class="pill bad">${t("lỗi")}</span>`}
      <${Icon} name="chevron" size=${12} class="tools-chev" />
    </button>
    ${open && html`<div class="tools-list">${items.map((e) => html`<${Ev} key=${e.seq} e=${e} />`)}</div>`}
  </div>`;
}

export function Turn({ turn }) {
  const gone = !turn.status;
  const { meta, events } = useRunStream(gone ? null : turn.run);
  const items = useMemo(() => foldTools(groupEvents(events).filter(shown)), [events]);
  const u = meta?.usage || {};
  return html`<div class="turn">
    ${turn.text ? html`<div class="msg-user"><div class="bubble user">${turn.text}</div></div>`
      : html`<div class="turn-from"><${Icon} name="zap" size=${12} />${t("Tiếp nối lượt chạy: {title}", { title: meta?.title || turn.run })}</div>`}
    ${gone ? html`<div class="small muted turn-gone">${t("Lượt này không còn trong lịch sử chạy.")}</div>` : html`<div class="turn-agent">
      ${items.map((e) => e.kind === "tools" ? html`<${ToolsFold} key=${e.seq} items=${e.items} live=${isActive(meta?.status)} />` : html`<${Ev} key=${e.seq} e=${e} />`)}
      ${meta && isActive(meta.status) && html`<div class="status-line"><span><${Icon} name="loader" class="spin" size=${12} /> ${meta.status === "queued" ? t("Đang chờ tới lượt") : t("đang làm việc")}</span></div>`}
      ${meta && !isActive(meta.status) && html`<div class="turn-foot"><${StatusPill} status=${meta.status} /><span>${runDuration(meta)}</span>
        ${u.costUsd ? html`<span>$${u.costUsd.toFixed(3)}</span>` : null}<span>${ago(meta.ended)}</span>
        <a onClick=${() => openRun(meta.id)}>${t("chi tiết")}</a></div>`}
    </div>`}
  </div>`;
}
