// Step 8: the lead sheet transcribed from the sung vocal.
import { api, html, media, t, toastError, useState } from "../core.js";
import { Btn, Empty, Lightbox, Seg } from "../ui.js";
import { runAction } from "./actions.js";
import { ActionCard } from "./parts.js";

const SHEET_KEYS = [["orig", t("Như giọng hát")], ["C", t("Đô (C)")], ["D", t("Rê (D)")], ["E", t("Mi (E)")], ["F", t("Fa (F)")], ["G", t("Sol (G)")], ["A", t("La (A)")], ["Bb", t("Si giáng (Bb)")], ["Eb", t("Mi giáng (Eb)")]];

// SheetStep writes a lead sheet (melody + lyrics) from the sung vocal, like a printed song sheet.
export function SheetStep({ d, slug, stamp }) {
  const sh = d.sheet;
  const [meter, setMeter] = useState(sh?.meter || "4/4");
  const [key, setKey] = useState(sh?.targetKey || "orig");
  const [box, setBox] = useState(null);
  const open = (path, extra = {}) => api(`/api/projects/${encodeURIComponent(slug)}/open`, { method: "POST", body: { path, ...extra } }).catch(toastError);
  return html`<div class="stack">
    ${!d.timings.length && html`<div class="warn-box">${t("Cần canh lời trước (")}<a href=${`#/p/${slug}/align`} style="color:inherit;text-decoration:underline">${t("bước 4")}</a>${t("): nhịp của bản nhạc lấy từ thời điểm hát từng chữ.")}</div>`}
    <${ActionCard} title=${t("Chép bản nhạc")} hint=${t("Lấy cao độ từ giọng hát đã tách và tiết tấu từ thời điểm từng chữ, viết thành bản nhạc có lời như bản in. Bản tự động, nên mở bằng MuseScore để sửa vài chỗ trước khi in")}
      onRun=${() => runAction(slug, "sheet", { meter, key })} label=${sh ? t("Chép lại") : t("Chép bản nhạc")} icon="sheet">
      <div class="row wrap" style="gap:8px">
        <${Seg} value=${meter} onChange=${setMeter} options=${["2/4", "3/4", "4/4"].map((m) => ({ value: m, label: m }))} />
        <select class="select" value=${key} onChange=${(e) => setKey(e.target.value)} aria-label=${t("Giọng")}>
          ${SHEET_KEYS.map(([v, l]) => html`<option value=${v}>${v === "orig" ? l : t("Dịch sang {key}", { key: l })}</option>`)}
        </select>
      </div>
    </${ActionCard}>
    ${sh ? html`
      <div class="row wrap" style="gap:8px">
        <span class="pill">${t("Giọng {key}", { key: sh.key })}</span><span class="pill">${t("Nhịp {meter}", { meter: sh.meter })}</span><span class="pill">♩ = ${Math.round(sh.bpm)}</span>
        <span class="pill" title=${t("Phần giọng hát mà nốt viết ra khớp trong vòng nửa cung")}>${t("Khớp cao độ {n}%", { n: Math.round(sh.pitchMatch * 100) })}</span>
      </div>
      <div class="row wrap">
        ${sh.pdf && html`<${Btn} size="sm" icon="sheet" onClick=${() => open(sh.pdf)}>${t("Mở PDF")}</${Btn}>`}
        <${Btn} size="sm" icon="edit" onClick=${() => open(sh.xml, { app: "musescore" })}>${t("Sửa trong MuseScore")}</${Btn}>
        <${Btn} size="sm" icon="folder" onClick=${() => open(sh.pdf || sh.xml, { reveal: true })}>${t("Hiện trong Finder")}</${Btn}>
      </div>
      ${sh.pages.length ? html`<div class="sheet-pages">${sh.pages.map((p, i) => html`<figure onClick=${() => setBox(p)}><img loading="lazy" decoding="async" src=${media(slug, p, stamp)} alt=${t("Trang {n}", { n: i + 1 })} /><figcaption>${t("Trang {n}", { n: i + 1 })}</figcaption></figure>`)}</div>`
        : html`<div class="muted small">${t("Chưa có ảnh xem trước: cài MuseScore (brew install --cask musescore) rồi chép lại để có PDF.")}</div>`}
      <div class="muted small">${t("Sửa xong trong MuseScore, dùng File → Export → PDF để in. Chép lại ở đây sẽ ghi đè bản đã sửa.")}</div>`
      : html`<${Empty} icon="sheet" title=${t("Chưa có bản nhạc")}>${t("Bản nhạc được chép tự động khi chạy chuỗi video, hoặc bấm Chép bản nhạc.")}</${Empty}>`}
    ${box && html`<${Lightbox} src=${media(slug, box, stamp)} caption=${box} onClose=${() => setBox(null)} />`}
  </div>`;
}
