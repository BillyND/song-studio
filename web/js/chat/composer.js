// The message box: Enter sends or queues, Cmd+Enter stops the turn and sends.
import { html, t, useEffect, useLayoutEffect, useRef } from "../core.js";
import { Btn } from "../ui.js";

export function Composer({ value, onChange, onSend, onSendNow, running, busy, autofocus }) {
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
