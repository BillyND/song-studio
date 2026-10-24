// Live run console: subscribes to a run's SSE stream and renders a readable timeline.
import { html, useState, useEffect, useRef, useMemo, api, store, toastError, bumpRuns, runDuration, go, t, lang, openChat, where } from "./core.js";
import { Icon } from "./icons.js";
import { Btn, StatusPill, AgentIcon, Markdown, ResizeHandle } from "./ui.js";

const active = (s) => s === "queued" || s === "running";

export function useRunStream(id) {
  const [meta, setMeta] = useState(null);
  const [events, setEvents] = useState([]);
  useEffect(() => {
    if (!id) return;
    setMeta(null);
    setEvents([]);
    let last = 0;
    let es = null;
    let closed = false;
    let wasActive = false;
    const connect = () => {
      es = new EventSource(`/api/runs/${encodeURIComponent(id)}/events?after=${last}`);
      es.onmessage = (m) => {
        const e = JSON.parse(m.data);
        if (e.seq <= last) return;
        if (e.seq !== last + 1) { es.close(); setTimeout(() => !closed && connect(), 100); return; }
        last = e.seq;
        setEvents((list) => [...list, e]);
      };
      es.addEventListener("meta", (m) => {
        const next = JSON.parse(m.data);
        if (active(next.status)) wasActive = true;
        setMeta(next);
      });
      es.addEventListener("end", (m) => {
        setMeta(JSON.parse(m.data));
        closed = true;
        es.close();
        if (wasActive) bumpRuns();
      });
      es.onerror = () => {
        if (closed) return;
        es.close();
        setTimeout(() => !closed && connect(), 1500); // the server may have restarted; resume after the last seq
      };
    };
    connect();
    return () => { closed = true; es && es.close(); };
  }, [id]);
  return { meta, events };
}

// Pair tool calls with their results and merge consecutive log lines into one block.
export function groupEvents(events) {
  const out = [];
  const byRef = new Map();
  let lastText = "";
  for (const e of events) {
    if (e.kind === "text") lastText = e.body;
    if (e.kind === "result" && e.body && e.body.trim() === lastText.trim()) {
      out.push({ ...e, body: "" }); // the CLI repeats its final reply in the result; show it once
      continue;
    }
    if (e.kind === "tool_result" && e.ref && byRef.has(e.ref)) {
      byRef.get(e.ref).result = e;
      continue;
    }
    if (e.kind === "log" || e.kind === "stderr") {
      const prev = out[out.length - 1];
      if (prev && prev.kind === "logs") { prev.lines.push(e); continue; }
      out.push({ kind: "logs", seq: e.seq, lines: [e] });
      continue;
    }
    const item = { ...e };
    if (e.kind === "tool_call" && e.ref) byRef.set(e.ref, item);
    out.push(item);
  }
  return out;
}

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
      if (!c.some((r) => active(r.status))) clearInterval(t);
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
    if (!meta || !active(meta.status)) return;
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
        ${active(meta.status) && html`<${Btn} kind="danger" size="sm" icon="stop" onClick=${stop}>${t("Dừng")}</${Btn}>`}
        ${meta.chat ? html`<${Btn} size="sm" icon="bot" onClick=${() => openChat(meta.chat)}>${t("Mở chat")}</${Btn}>`
          : meta.kind === "agent" && !active(meta.status) && meta.usage?.session && html`<${Btn} size="sm" icon="send" title=${t("Nói tiếp với agent trong phiên này")} onClick=${() => replyTo(id)}>${t("Trả lời")}</${Btn}>`}
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
        ${meta.exitCode != null && !active(meta.status) ? html`<span>${t("mã thoát")} <b>${meta.exitCode}</b></span>` : null}
        ${meta.prompt && html`<a style="cursor:pointer;color:var(--accent)" onClick=${() => setShowPrompt(!showPrompt)}>${showPrompt ? t("ẩn prompt") : t("xem prompt")}</a>`}
      </div>
      <${ChainBar} group=${meta.group} current=${id} follow=${!!compact} />
      ${showPrompt && html`<pre class="block" style="max-height:220px">${meta.prompt}</pre>`}
    </div>
    <div class="timeline" ref=${box} onScroll=${(e) => { const el = e.currentTarget; stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 60; }}>
      ${items.length === 0 && html`<div class="empty small">${meta.status === "queued" ? t("Đang chờ tới lượt, máy chỉ chạy số việc nặng đã đặt trong Cài đặt.") : t("Đang khởi động…")}</div>`}
      ${items.map((e) => html`<${Ev} key=${e.seq} e=${e} />`)}
      ${active(meta.status) && items.length > 0 && html`<div class="status-line"><${Icon} name="loader" class="spin" size=${12} /> ${t("đang làm việc")}</div>`}
    </div>
  </div>`;
}

export function Drawer() {
  const id = store.get().consoleRun;
  return id ? html`<aside class="drawer"><${ResizeHandle} /><${RunView} key=${id} id=${id} compact onClose=${() => store.set({ consoleRun: null })} /></aside>` : null;
}
