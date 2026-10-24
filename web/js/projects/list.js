// The song list: one card per song with its cover and progress.
import { ago, api, go, html, media, t, toastError, useEffect, useState, useStore } from "../core.js";
import { Icon } from "../icons.js";
import { Btn, Empty } from "../ui.js";
import { NewSong } from "./new-song.js";

export function ProjectsPage() {
  const [list, setList] = useState(null);
  const [creating, setCreating] = useState(false);
  const runsVersion = useStore((s) => s.runsVersion);
  useEffect(() => { api("/api/projects").then(setList).catch(toastError); }, [runsVersion]);
  return html`<div class="page">
    <div class="page-head">
      <div><div class="eyebrow">Song Studio</div><h1>${t("Bài hát")}</h1>
        <div class="muted" style="margin-top:6px">${t("Mỗi bài, thuộc thể loại nào cũng vậy, đi qua 8 bước: từ lời tới bài hát, video karaoke và bản nhạc. Bấm vào một bài để làm tiếp.")}</div></div>
      <${Btn} kind="primary" icon="plus" onClick=${() => setCreating(true)}>${t("Bài mới")}</${Btn}>
    </div>
    ${list === null ? html`<div class="empty"><${Icon} name="loader" class="spin" size=${22} /></div>` : html`
    <div class="grid-cards">
      ${list.map((p) => html`<${SongCard} key=${p.slug} p=${p} />`)}
      <div class="card song new-card" onClick=${() => setCreating(true)}><div style="text-align:center"><${Icon} name="plus" size=${26} /><div style="margin-top:8px;font-weight:600">${t("Tạo bài mới từ lời")}</div></div></div>
    </div>`}
    ${list && list.length === 0 && html`<${Empty} icon="lotus" title=${t("Chưa có bài nào")}>${t("Bấm \"Bài mới\" và dán lời bài hát để bắt đầu.")}</${Empty}>`}
    ${creating && html`<${NewSong} onClose=${() => setCreating(false)} />`}
  </div>`;
}

function SongCard({ p }) {
  const pct = p.total ? Math.round((p.done / p.total) * 100) : 0;
  const initials = (p.title || "?").split(/\s+/).map((w) => w[0]).join("").slice(0, 3).toUpperCase();
  return html`<div class="card song" onClick=${() => go("/p/" + encodeURIComponent(p.slug))} role="link" tabindex="0" onKeyDown=${(e) => e.key === "Enter" && go("/p/" + p.slug)}>
    <div class="song-cover">
      ${p.cover ? html`<img loading="lazy" decoding="async" src=${media(p.slug, p.cover)} alt="" />` : html`<div class="ph">${initials}</div>`}
      <div class="song-badge">${p.video ? html`<span class="pill ok"><${Icon} name="film" size=${11} /> ${t("Có video")}</span>` : html`<span class="pill">${t("{done}/{total} bước", { done: p.done, total: p.total })}</span>`}</div>
    </div>
    <div class="song-meta">
      <div class="song-title">${p.title}</div>
      <div class="muted small">${p.subtitle || "—"} · ${ago(p.updated)}</div>
      <div class="progress"><i style=${{ width: pct + "%" }}></i></div>
    </div>
  </div>`;
}
