// Saved chats, newest first, with delete.
import { ago, api, html, isActive, openChat, store, t, toastError, useEffect, useState } from "../core.js";
import { Icon } from "../icons.js";
import { AgentIcon, Btn, StatusPill } from "../ui.js";
import { contextOf, useTitles } from "./context.js";

export function ChatList({ current, onPick }) {
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
        ${isActive(c.status) && html`<${StatusPill} status=${c.status} />`}
        <${Btn} kind="ghost" size="sm" icon="trash" title=${t("Xoá cuộc trò chuyện")} onClick=${(e) => del(e, c)} />
      </div>`;
    })}
  </div></div>`;
}
