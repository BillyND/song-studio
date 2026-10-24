// Agent chat panel: multi-turn conversations with a coding agent, docked on the right of every
// page. Each message is an agent run that resumes the session of the turn before it.
import { html, useState, useEffect, useRef, useMemo, useLayoutEffect, api, store, useStore, useRoute, toastError, bumpRuns, t, ago, runDuration, openChat, closeChat, openRun, where } from "./core.js";
import { Icon } from "./icons.js";
import { Btn, StatusPill, AgentIcon, AgentPicker, useAgentChoice, PresetChips, ResizeHandle } from "./ui.js";
import { useRunStream, groupEvents, Ev } from "./console.js";

const active = (s) => s === "queued" || s === "running";
const RECENT_TURNS = 8;

// Song titles for context chips, fetched once per page load.
let titlesCache = null;
function useTitles() {
  const [titles, setTitles] = useState(titlesCache || {});
  useEffect(() => {
    if (titlesCache) return;
    api("/api/projects").then((list) => {
      titlesCache = Object.fromEntries(list.map((p) => [p.slug, p.title]));
      setTitles(titlesCache);
    }).catch(() => {});
  }, []);
  return titles;
}

function contextOf(c, titles) {
  if (c.target === "project") return { icon: "music", label: titles[c.project] || c.project };
  if (c.target === "skill") return { icon: "code", label: "Skill" };
  return { icon: "folder", label: t("Mọi bài hát") };
}

function ContextChip({ ctx, onRemove }) {
  const titles = useTitles();
  const c = contextOf(ctx, titles);
  return html`<span class="ctx-chip" title=${t("Agent làm việc trong thư mục này")}><${Icon} name=${c.icon} size=${12} /><span class="ellipsis">${c.label}</span>
    ${onRemove && html`<button type="button" title=${t("Bỏ ngữ cảnh này")} onClick=${onRemove}><${Icon} name="x" size=${11} /></button>`}</span>`;
}

const SUGGESTIONS = {
  project: ["Bài này đang ở bước nào, nên làm gì tiếp?", "Soát lại lời: chính tả, dấu câu, chia đoạn", "Viết lại Styles cho bản phối mộc hơn, ít nhạc cụ"],
  skill: ["Tóm tắt các bước của skill", "Bước canh lời hoạt động thế nào?"],
  songs: ["Bài nào chưa có video?", "Liệt kê các bài và bước đang dừng"],
};

export function ChatPanel() {
  const id = useStore((s) => s.chatId);
  const [list, setList] = useState(false);
  useEffect(() => setList(false), [id]);
  return html`<aside class="drawer chat"><${ResizeHandle} />
    <div class="chat-head">
      <span class="chat-mark"><${Icon} name="bot" size=${15} /></span><b class="grow">Agent</b>
      <${Btn} kind="ghost" size="sm" icon="list" title=${t("Các cuộc trò chuyện")} onClick=${() => setList(!list)} />
      <${Btn} kind="ghost" size="sm" icon="plus" title=${t("Cuộc trò chuyện mới")} onClick=${() => { setList(false); openChat(null); }} />
      <${Btn} kind="ghost" size="sm" icon="x" title=${t("Đóng (⌘J)")} onClick=${closeChat} />
    </div>
    ${list ? html`<${ChatList} current=${id} onPick=${(c) => { setList(false); openChat(c); }} />`
      : id ? html`<${Thread} key=${id} id=${id} />` : html`<${NewChat} />`}
  </aside>`;
}

function NewChat() {
  const route = useRoute();
  const here = route[0] === "p" && route[1] ? { target: "project", project: route[1] } : route[0] === "skill" ? { target: "skill" } : { target: "songs" };
  const [dropped, setDropped] = useState(false);
  useEffect(() => setDropped(false), [here.target, here.project]);
  const ctx = dropped ? { target: "songs" } : here;
  const ag = useAgentChoice();
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const send = async () => {
    if (!text.trim() || busy) return;
    setBusy(true);
    try {
      const c = await api("/api/chats", { method: "POST", body: { ...ctx, agent: ag.agent, model: ag.model, text, where: where() } });
      setText(""); bumpRuns(); openChat(c.id);
    } catch (e) { toastError(e); } finally { setBusy(false); }
  };
  return html`<div class="chat-body">
    <div class="chat-scroll"><div class="chat-intro">
      <div class="chat-hello"><span class="chat-mark big"><${Icon} name="sparkles" size=${20} /></span>
        <h3>${t("Trò chuyện với agent")}</h3>
        <p class="muted small">${t("Agent làm trong thư mục bên dưới, nhớ cả cuộc trò chuyện và có thể hỏi lại khi chưa rõ. Nó biết bạn đang mở màn nào, nên cứ nói \"bài này\", \"file này\".")}</p></div>
      <div class="chat-field"><span class="small muted">${t("Làm việc với")}</span>
        <${ContextChip} ctx=${ctx} onRemove=${ctx.target !== "songs" ? () => setDropped(true) : null} /></div>
      <div class="chat-field"><span class="small muted">Agent</span><${AgentPicker} value=${ag.agent} onChange=${ag.setAgent} model=${ag.model} onModel=${ag.setModel} /></div>
      <div class="chat-suggest">${SUGGESTIONS[ctx.target].map((s) => html`<button type="button" onClick=${() => setText(t(s))}>${t(s)}</button>`)}</div>
      ${ctx.target === "project" && html`<${PresetChips} value=${text} onChange=${setText} />`}
    </div></div>
    <${Composer} value=${text} onChange=${setText} onSend=${send} busy=${busy} autofocus />
  </div>`;
}

function Thread({ id }) {
  const [chat, setChat] = useState(null);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [showAll, setShowAll] = useState(false);
  const box = useRef(null);
  const inner = useRef(null);
  const stick = useRef(true);
  const ver = useRef(0); // a poll that started before a send must not undo what the send returned
  useEffect(() => {
    let stop = false;
    const load = () => {
      const mine = ++ver.current;
      api(`/api/chats/${encodeURIComponent(id)}`).then((c) => !stop && mine === ver.current && setChat(c)).catch((e) => {
        if (!stop && e.status === 404) openChat(null); // deleted elsewhere
      });
    };
    load();
    // quick while a turn runs or messages wait, slow otherwise (another window may still add a turn)
    let idle = 0;
    const iv = setInterval(() => {
      if (document.hidden) return;
      const c = latest.current;
      if (c && !active(c.status) && !c.queue.length && ++idle % 8) return;
      load();
    }, 2000);
    return () => { stop = true; clearInterval(iv); };
  }, [id]);
  const latest = useRef(null);
  latest.current = chat;
  useLayoutEffect(() => {
    // keep the newest output in view while the user has not scrolled up
    const el = box.current, content = inner.current;
    if (!el || !content) return;
    const ro = new ResizeObserver(() => { if (stick.current) el.scrollTop = el.scrollHeight; });
    ro.observe(content);
    return () => ro.disconnect();
  }, [!!chat]);
  if (!chat) return html`<div class="empty"><${Icon} name="loader" class="spin" size=${22} /></div>`;
  const running = active(chat.status);
  // older turns stay unloaded (no stream, no events in memory) until asked for
  const hidden = showAll ? 0 : Math.max(0, chat.turns.length - RECENT_TURNS);
  const last = chat.turns[chat.turns.length - 1];
  const send = async (now) => {
    const msg = text.trim();
    if (!msg || busy) return;
    setBusy(true);
    try {
      ++ver.current;
      const c = await api(`/api/chats/${encodeURIComponent(id)}/messages`, { method: "POST", body: { text: msg, now, where: where() } });
      ++ver.current;
      setChat(c);
      setText(""); stick.current = true; bumpRuns();
    } catch (e) { toastError(e); } finally { setBusy(false); }
  };
  const stopTurn = () => api(`/api/runs/${encodeURIComponent(last.run)}/stop`, { method: "POST" }).catch(toastError);
  const unqueue = (i) => { ++ver.current; api(`/api/chats/${encodeURIComponent(id)}/queue/${i}`, { method: "DELETE" }).then((c) => { ++ver.current; setChat(c); }).catch(toastError); };
  return html`<div class="chat-body">
    <div class="chat-sub">
      <div class="chat-title ellipsis" title=${chat.title}>${chat.title}</div>
      <div class="row" style="gap:6px"><${ContextChip} ctx=${chat} /><span class="ctx-chip"><${AgentIcon} agent=${chat.agent} />${chat.agent}${chat.model ? ` · ${chat.model}` : ""}</span>
        <span class="spacer"></span>${running && html`<${Btn} kind="ghost" size="sm" icon="stop" onClick=${stopTurn}>${t("Dừng")}</${Btn}>`}</div>
    </div>
    <div class="chat-scroll" ref=${box} onScroll=${(e) => { const el = e.currentTarget; stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 60; }}>
      <div class="chat-turns" ref=${inner}>
        ${hidden > 0 && html`<button type="button" class="older-turns" onClick=${() => setShowAll(true)}>${t("Hiện {n} lượt trước", { n: hidden })}</button>`}
        ${chat.turns.slice(hidden).map((tn) => html`<${Turn} key=${tn.run} turn=${tn} />`)}</div>
    </div>
    ${chat.queue.length > 0 && html`<div class="chat-queue">
      ${chat.error && !running ? html`<div class="small queue-err"><${Icon} name="alert" size=${12} /> ${t("Chưa gửi được: {error}. Gửi thêm một tin để thử lại, hoặc bỏ các tin này.", { error: chat.error })}</div>`
        : html`<div class="small muted"><${Icon} name="clock" size=${12} /> ${t("Chờ gửi khi agent xong lượt này")}</div>`}
      ${chat.queue.map((q, i) => html`<div class="queued"><span class="ellipsis">${q}</span>
        <button type="button" title=${t("Bỏ tin này")} onClick=${() => unqueue(i)}><${Icon} name="x" size=${12} /></button></div>`)}</div>`}
    <${Composer} value=${text} onChange=${setText} running=${running} busy=${busy} onSend=${() => send(false)} onSendNow=${() => send(true)} />
  </div>`;
}

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
      <b>${calls.length ? t("{n} thao tác", { n: calls.length }) : t("Nhật ký")}</b>
      <span class="muted ellipsis">${running && last ? last.title || last.tool : names}</span>
      ${failed && html`<span class="pill bad">${t("lỗi")}</span>`}
      <${Icon} name="chevron" size=${12} class="tools-chev" />
    </button>
    ${open && html`<div class="tools-list">${items.map((e) => html`<${Ev} key=${e.seq} e=${e} />`)}</div>`}
  </div>`;
}

function Turn({ turn }) {
  const gone = !turn.status;
  const { meta, events } = useRunStream(gone ? null : turn.run);
  const items = useMemo(() => foldTools(groupEvents(events).filter(shown)), [events]);
  const u = meta?.usage || {};
  return html`<div class="turn">
    ${turn.text ? html`<div class="msg-user"><div class="bubble user">${turn.text}</div></div>`
      : html`<div class="turn-from"><${Icon} name="zap" size=${12} />${t("Tiếp nối lượt chạy: {title}", { title: meta?.title || turn.run })}</div>`}
    ${gone ? html`<div class="small muted turn-gone">${t("Lượt này không còn trong lịch sử chạy.")}</div>` : html`<div class="turn-agent">
      ${items.map((e) => e.kind === "tools" ? html`<${ToolsFold} key=${e.seq} items=${e.items} live=${active(meta?.status)} />` : html`<${Ev} key=${e.seq} e=${e} />`)}
      ${meta && active(meta.status) && html`<div class="status-line"><span><${Icon} name="loader" class="spin" size=${12} /> ${meta.status === "queued" ? t("Đang chờ tới lượt") : t("đang làm việc")}</span></div>`}
      ${meta && !active(meta.status) && html`<div class="turn-foot"><${StatusPill} status=${meta.status} /><span>${runDuration(meta)}</span>
        ${u.costUsd ? html`<span>$${u.costUsd.toFixed(3)}</span>` : null}<span>${ago(meta.ended)}</span>
        <a onClick=${() => openRun(meta.id)}>${t("chi tiết")}</a></div>`}
    </div>`}
  </div>`;
}

function Composer({ value, onChange, onSend, onSendNow, running, busy, autofocus }) {
  const ref = useRef(null);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 200) + "px";
  }, [value]);
  useEffect(() => { if (autofocus) ref.current?.focus(); }, []);
  const key = (e) => {
    // isComposing: Enter that confirms a Vietnamese (Telex/VNI) or other IME word must not send
    if (e.key !== "Enter" || e.shiftKey || e.isComposing || e.keyCode === 229) return;
    e.preventDefault();
    if ((e.metaKey || e.ctrlKey) && running && onSendNow) onSendNow();
    else onSend();
  };
  const empty = !value.trim();
  return html`<div class="composer">
    <textarea ref=${ref} rows="1" class="composer-input" value=${value} onInput=${(e) => onChange(e.target.value)} onKeyDown=${key}
      placeholder=${running ? t("Nhắn thêm, tin sẽ chờ tới khi agent xong lượt này…") : t("Nhắn agent…")}></textarea>
    <div class="composer-bar">
      <span class="muted small hint">${running ? t("Enter: xếp hàng · ⌘Enter: dừng & gửi ngay") : t("Enter để gửi · Shift+Enter xuống dòng")}</span>
      <span class="spacer"></span>
      ${running && onSendNow && html`<${Btn} size="sm" icon="stop" disabled=${empty} busy=${busy} title=${t("Dừng lượt đang chạy và gửi tin này ngay")} onClick=${onSendNow}>${t("Dừng & gửi")}</${Btn}>`}
      <${Btn} size="sm" kind="primary" icon="send" disabled=${empty} busy=${busy} onClick=${onSend}>${running ? t("Xếp hàng") : t("Gửi")}</${Btn}>
    </div>
  </div>`;
}

function ChatList({ current, onPick }) {
  const [chats, setChats] = useState(null);
  const titles = useTitles();
  const load = () => api("/api/chats").then(setChats).catch(toastError);
  useEffect(() => { load(); }, []);
  const del = async (e, c) => {
    e.stopPropagation();
    if (!confirm(t("Xoá cuộc trò chuyện \"{title}\"? Các lượt chạy vẫn còn trong mục Lượt chạy.", { title: c.title }))) return;
    try {
      await api(`/api/chats/${encodeURIComponent(c.id)}`, { method: "DELETE" });
      if (store.get().chatId === c.id) openChat(null);
      load();
    } catch (err) { toastError(err); }
  };
  if (!chats) return html`<div class="empty"><${Icon} name="loader" class="spin" size=${22} /></div>`;
  return html`<div class="chat-scroll"><div class="chat-list">
    ${chats.length === 0 && html`<div class="empty small">${t("Chưa có cuộc trò chuyện nào.")}</div>`}
    ${chats.map((c) => {
      const ctx = contextOf(c, titles);
      return html`<div role="button" tabindex="0" class=${"chat-item" + (c.id === current ? " on" : "")} onClick=${() => onPick(c.id)} onKeyDown=${(e) => e.key === "Enter" && onPick(c.id)}>
        <${AgentIcon} agent=${c.agent} />
        <div class="grow" style="min-width:0"><div class="ellipsis chat-item-title">${c.title}</div>
          <div class="small muted ellipsis"><${Icon} name=${ctx.icon} size=${11} /> ${ctx.label} · ${c.turns.length === 1 ? t("1 lượt") : t("{n} lượt", { n: c.turns.length })} · ${ago(c.updated)}</div></div>
        ${active(c.status) && html`<${StatusPill} status=${c.status} />`}
        <${Btn} kind="ghost" size="sm" icon="trash" title=${t("Xoá cuộc trò chuyện")} onClick=${(e) => del(e, c)} />
      </div>`;
    })}
  </div></div>`;
}
