// Step 2: the Suno prompt (Lyrics and Styles) ready to copy, edit or have an agent fill in.
import { html, t } from "../core.js";
import { Icon } from "../icons.js";
import { Btn, Empty, Markdown } from "../ui.js";
import { safeJSON, useSave } from "./actions.js";
import { AgentAction, ChainStatus, CopyBlock } from "./parts.js";

export function SunoStep({ d, slug, reload }) {
  const [busy, save] = useSave(slug, reload);
  const cfg = safeJSON(d.config);
  const n = [...(d.sunoStyle || "").trim()].length;
  const empty = !d.sunoLyrics && !d.sunoStyle;
  return html`<div class="stack">
    <${ChainStatus} slug=${slug} steps=${["lyrics", "suno"]} title=${t("Chuỗi tạo prompt")} />
    ${empty ? html`<${Empty} icon="sparkles" title=${t("Chưa có prompt Suno")}>${t("Ở")} <a href=${`#/p/${slug}/lyrics`} style="color:var(--accent-text)">${t("bước 1")}</a>${t(", thêm lời (và bài mẫu, yêu cầu nếu có) rồi bấm")} <b>${t("Tạo prompt Suno")}</b>.</${Empty}>` : html`
      <dl class="kv"><dt>${t("Tên bài")}</dt><dd><b>${[cfg.title, cfg.subtitle].filter(Boolean).join(" ")}</b></dd><dt>${t("Giọng hát")}</dt><dd>${cfg.vocal || "—"}</dd></dl>
      <${CopyBlock} title="Lyrics" text=${(d.sunoLyrics || "").trim()} saving=${busy} onSave=${(v) => save("suno/lyrics.txt", v)} />
      <${CopyBlock} title="Styles" text=${(d.sunoStyle || "").trim()} extra=${html`<span class=${"pill " + (n > 1000 ? "bad" : "ok")}>${n}/1000</span>`} saving=${busy} onSave=${(v) => save("suno/style.txt", v)} />
      ${d.notes && html`<details><summary class="section-title" style="cursor:pointer"><${Icon} name="file" /><h3>${t("Ghi chú soát lời")}</h3></summary><div class="bubble" style="margin-top:8px"><${Markdown} text=${d.notes} /></div></details>`}
      <${AgentAction} slug=${slug} action="suno.agent" title=${t("Điền form suno.com/create")} hint=${t("Agent dùng CUA điều khiển Chrome; không bấm Create, bạn tự xem và bấm")} />
      <div class="row wrap"><${Btn} icon="external" onClick=${() => window.open("https://suno.com/create", "_blank", "noopener")}>${t("Mở Suno")}</${Btn}>
        <span class="muted small">${t("Tạo xong, tải bản")} <b>M4A</b> ${t("(có sẵn timing lời) rồi sang bước Nhạc.")}</span></div>
    `}
  </div>`;
}
