// Starting a chat: where the agent works, which agent, suggestions, first message.
import { api, bumpRuns, html, openChat, t, toastError, useEffect, useRoute, useState, where } from "../core.js";
import { Icon } from "../icons.js";
import { AgentPicker, PresetChips, useAgentChoice } from "../ui.js";
import { Composer } from "./composer.js";
import { ContextChip } from "./context.js";

const SUGGESTIONS = {
  project: ["Bài này đang ở bước nào, nên làm gì tiếp?", "Soát lại lời: chính tả, dấu câu, chia đoạn", "Viết lại Styles cho bản phối mộc hơn, ít nhạc cụ"],
  skill: ["Tóm tắt các bước của skill", "Bước canh lời hoạt động thế nào?"],
  songs: ["Bài nào chưa có video?", "Liệt kê các bài và bước đang dừng"],
};

export function NewChat() {
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
