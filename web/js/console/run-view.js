// The run console: header with stats and actions, the chain of linked runs, the timeline, the drawer.
import { api, go, html, isActive, openChat, runDuration, store, t, toastError, useEffect, useMemo, useRef, useState, where } from "../core.js";
import { Icon } from "../icons.js";
import { AgentIcon, Btn, ResizeHandle, StatusPill } from "../ui.js";
import { Ev } from "./events.js";
import { groupEvents, useRunStream } from "./stream.js";

function ChainBar({ group, current, follow }) {
  const [chain, setChain] = useState([]);
  const prev = useRef(null);
  useEffect(() => {
    setChain([]);
    if (!group) return;
    let stop = false;
    // poll while some link of the chain can still change; a finished chain is final
    const load = () => api(`/api/groups/${encodeURIComponent(group)}`).then((c) => {
      if (stop) return;
      setChain(c);
      if (!c.some((r) => isActive(r.status))) clearInterval(t);
    }).catch(() => {});
    const t = setInterval(() => !document.hidden && load(), 2000);
    load();
    return () => { stop = true; clearInterval(t); };
  }, [group]);
  useEffect(() => {
    // follow only when the run being watched finishes and the chain moves on; a link the user picked stays
    const me = chain.find((r) => r.id === current);
    const was = prev.current;
    prev.current = me ? me.status : null;
    if (!follow || !me || was !== "running" || me.status !== "succeeded") return;
    const live = chain.find((r) => r.status === "running");
    if (live && live.id !== current) store.set({ consoleRun: live.id });
  }, [chain, current]);
  if (chain.length < 2) return null;
  return html`<div class="chain tight">${chain.map((r, i) => html`${i > 0 && html`<${Icon} name="chevron" size=${11} class="muted" />`}
    <button type="button" class=${"chain-step" + (r.id === current ? " on" : "")} onClick=${() => store.set({ consoleRun: r.id })} title=${r.title}>
      <${AgentIcon} agent=${r.kind === "agent" ? r.agent : "script"} /><span>${r.title}</span><${StatusPill} status=${r.status} /></button>`)}</div>`;
}

// replyTo continues a finished agent run as a chat (or reopens the chat that already does).
export async function replyTo(id) {
  try { const c = await api("/api/chats", { method: "POST", body: { fromRun: id, where: where() } }); openChat(c.id); }
  catch (e) { toastError(e); }
}

export function RunView({ id, compact, onClose }) {
  const { meta, events } = useRunStream(id);
  const [, tick] = useState(0);
  const [showPrompt, setShowPrompt] = useState(false);
  const box = useRef(null);
  const stick = useRef(true);
  const items = useMemo(() => groupEvents(events), [events]);

  useEffect(() => {
    if (!meta || !isActive(meta.status)) return;
    const t = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, [meta?.status]);
  useEffect(() => {
    const el = box.current;
    if (el && stick.current) el.scrollTop = el.scrollHeight;
  }, [items.length, events.length]);

  if (!meta) return html`<div class="empty"><${Icon} name="loader" class="spin" size=${22} /></div>`;
  const stop = async () => {
    try { await api(`/api/runs/${id}/stop`, { method: "POST" }); } catch (e) { toastError(e); }
  };
  const u = meta.usage || {};
  return html`<div style="display:flex;flex-direction:column;height:100%;min-height:0;position:relative">
    <div class="console-head">
      <div class="row" style="align-items:flex-start">
        <${AgentIcon} agent=${meta.kind === "agent" ? meta.agent : "script"} />
        <div class="grow"><div class="console-title">${meta.title || meta.id}</div>
          ${meta.project && html`<a class="small muted" href=${"#/p/" + encodeURIComponent(meta.project)}>${meta.project}</a>`}</div>
        ${isActive(meta.status) && html`<${Btn} kind="danger" size="sm" icon="stop" onClick=${stop}>${t("Dừng")}</${Btn}>`}
        ${meta.chat ? html`<${Btn} size="sm" icon="bot" onClick=${() => openChat(meta.chat)}>${t("Mở chat")}</${Btn}>`
          : meta.kind === "agent" && !isActive(meta.status) && meta.usage?.session && html`<${Btn} size="sm" icon="send" title=${t("Nói tiếp với agent trong phiên này")} onClick=${() => replyTo(id)}>${t("Trả lời")}</${Btn}>`}
        ${compact && html`<${Btn} kind="ghost" size="sm" icon="external" title=${t("Mở toàn trang")} onClick=${() => go("/runs/" + id)} />`}
        ${onClose && html`<${Btn} kind="ghost" size="sm" icon="x" title=${t("Đóng")} onClick=${onClose} />`}
      </div>
      <div class="console-stats">
        <${StatusPill} status=${meta.status} />
        <span><${Icon} name="clock" size=${12} /> <b>${runDuration(meta) || "—"}</b></span>
        ${meta.pid ? html`<span>PID <b>${meta.pid}</b></span>` : null}
        ${u.model && html`<span>model <b>${u.model}</b></span>`}
        ${(u.inputTokens || u.outputTokens) ? html`<span>token <b>${(u.inputTokens || 0).toLocaleString()}</b> / <b>${(u.outputTokens || 0).toLocaleString()}</b></span>` : null}
        ${u.costUsd ? html`<span>${t("chi phí")} <b>$${u.costUsd.toFixed(3)}</b></span>` : null}
        ${meta.exitCode != null && !isActive(meta.status) ? html`<span>${t("mã thoát")} <b>${meta.exitCode}</b></span>` : null}
        ${meta.prompt && html`<a style="cursor:pointer;color:var(--accent-text)" onClick=${() => setShowPrompt(!showPrompt)}>${showPrompt ? t("ẩn prompt") : t("xem prompt")}</a>`}
      </div>
      <${ChainBar} group=${meta.group} current=${id} follow=${!!compact} />
      ${showPrompt && html`<pre class="block" style="max-height:220px">${meta.prompt}</pre>`}
    </div>
    <div class="timeline" ref=${box} onScroll=${(e) => { const el = e.currentTarget; stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 60; }}>
      ${items.length === 0 && html`<div class="empty small">${meta.status === "queued" ? t("Đang chờ tới lượt, máy chỉ chạy số việc nặng đã đặt trong Cài đặt.") : t("Đang khởi động…")}</div>`}
      ${items.map((e) => html`<${Ev} key=${e.seq} e=${e} />`)}
      ${isActive(meta.status) && items.length > 0 && html`<div class="status-line"><${Icon} name="loader" class="spin" size=${12} /> ${t("đang làm việc")}</div>`}
    </div>
  </div>`;
}

export function Drawer() {
  const id = store.get().consoleRun;
  return id ? html`<aside class="drawer"><${ResizeHandle} /><${RunView} key=${id} id=${id} compact onClose=${() => store.set({ consoleRun: null })} /></aside>` : null;
}
