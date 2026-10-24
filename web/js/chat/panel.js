// The agent chat panel: header, chat list toggle, and the current or new conversation.
import { closeChat, html, openChat, t, useEffect, useState, useStore } from "../core.js";
import { Icon } from "../icons.js";
import { Btn, ResizeHandle } from "../ui.js";
import { ChatList } from "./list.js";
import { NewChat } from "./new-chat.js";
import { Thread } from "./thread.js";

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
