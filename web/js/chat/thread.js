// One conversation: its turns, queued messages and the composer, kept in sync with the server.
import { api, bumpRuns, html, isActive, openChat, t, toastError, useEffect, useLayoutEffect, useRef, useState, where } from "../core.js";
import { Icon } from "../icons.js";
import { AgentIcon, Btn } from "../ui.js";
import { Composer } from "./composer.js";
import { ContextChip, RECENT_TURNS } from "./context.js";
import { Turn } from "./turn.js";

export function Thread({ id }) {
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
      if (c && !isActive(c.status) && !c.queue.length && ++idle % 8) return;
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
  const running = isActive(chat.status);
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
