// Song list and the "new song" dialog.
import { html, useState, useEffect, api, toastError, go, ago, media, useStore, localStorageGet, localStorageSet } from "./core.js";
import { Icon } from "./icons.js";
import { Btn, Modal, Field, Empty } from "./ui.js";

export function ProjectsPage() {
  const [list, setList] = useState(null);
  const [creating, setCreating] = useState(false);
  const runsVersion = useStore((s) => s.runsVersion);
  useEffect(() => { api("/api/projects").then(setList).catch(toastError); }, [runsVersion]);
  return html`<div class="page">
    <div class="page-head">
      <div><div class="eyebrow">Quê Hương Studio</div><h1>Bài hát</h1>
        <div class="muted" style="margin-top:6px">Mỗi bài đi qua 8 bước, từ lời tới video karaoke. Bấm vào một bài để làm tiếp.</div></div>
      <${Btn} kind="primary" icon="plus" onClick=${() => setCreating(true)}>Bài mới</${Btn}>
    </div>
    ${list === null ? html`<div class="empty"><${Icon} name="loader" class="spin" size=${22} /></div>` : html`
    <div class="grid-cards">
      ${list.map((p) => html`<${SongCard} key=${p.slug} p=${p} />`)}
      <div class="card song new-card" onClick=${() => setCreating(true)}><div style="text-align:center"><${Icon} name="plus" size=${26} /><div style="margin-top:8px;font-weight:600">Tạo bài mới từ lời</div></div></div>
    </div>`}
    ${list && list.length === 0 && html`<${Empty} icon="lotus" title="Chưa có bài nào">Bấm "Bài mới" và dán lời bài hát để bắt đầu.</${Empty}>`}
    ${creating && html`<${NewSong} onClose=${() => setCreating(false)} />`}
  </div>`;
}

function SongCard({ p }) {
  const pct = p.total ? Math.round((p.done / p.total) * 100) : 0;
  const initials = (p.title || "?").split(/\s+/).map((w) => w[0]).join("").slice(0, 3).toUpperCase();
  return html`<div class="card song" onClick=${() => go("/p/" + encodeURIComponent(p.slug))} role="link" tabindex="0" onKeyDown=${(e) => e.key === "Enter" && go("/p/" + p.slug)}>
    <div class="song-cover">
      ${p.cover ? html`<img loading="lazy" src=${media(p.slug, p.cover)} alt="" />` : html`<div class="ph">${initials}</div>`}
      <div class="song-badge">${p.video ? html`<span class="pill ok"><${Icon} name="film" size=${11} /> Có video</span>` : html`<span class="pill">${p.done}/${p.total} bước</span>`}</div>
    </div>
    <div class="song-meta">
      <div class="song-title">${p.title}</div>
      <div class="muted small">${p.subtitle || "—"} · ${ago(p.updated)}</div>
      <div class="progress"><i style=${{ width: pct + "%" }}></i></div>
    </div>
  </div>`;
}

function NewSong({ onClose }) {
  const [form, setForm] = useState({
    title: "", subtitle: "", lyrics: "",
    composer: localStorageGet("composer") || "Nguyễn Văn Thanh",
    arranger: localStorageGet("arranger") || "Nguyễn Đức Long",
  });
  const [busy, setBusy] = useState(false);
  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });
  const submit = async () => {
    setBusy(true);
    try {
      localStorageSet("composer", form.composer);
      localStorageSet("arranger", form.arranger);
      const r = await api("/api/projects", { method: "POST", body: {
        title: form.title, subtitle: form.subtitle, lyrics: form.lyrics,
        credits: [["Sáng tác: ", form.composer], ["Phổ nhạc: ", form.arranger]],
      } });
      go("/p/" + encodeURIComponent(r.slug));
    } catch (e) { toastError(e); setBusy(false); }
  };
  const lines = form.lyrics.trim() ? form.lyrics.trim().split("\n").length : 0;
  return html`<${Modal} title="Bài hát mới" onClose=${onClose} footer=${html`
      <${Btn} kind="ghost" onClick=${onClose}>Huỷ</${Btn}>
      <${Btn} kind="primary" icon="sparkles" busy=${busy} disabled=${!form.title.trim() || !form.lyrics.trim()} onClick=${submit}>Tạo bài</${Btn}>`}>
    <div class="stack">
      <div class="row" style="gap:14px;align-items:flex-start">
        <div class="grow"><${Field} label="Tên bài (dòng lớn trên ảnh tựa đề)"><input class="input" placeholder="Ninh Bình" value=${form.title} onInput=${set("title")} autofocus /></${Field}></div>
        <div class="grow"><${Field} label="Dòng phụ"><input class="input" placeholder="Quê Hương" value=${form.subtitle} onInput=${set("subtitle")} /></${Field}></div>
      </div>
      <div class="row" style="gap:14px">
        <div class="grow"><${Field} label="Sáng tác"><input class="input" value=${form.composer} onInput=${set("composer")} /></${Field}></div>
        <div class="grow"><${Field} label="Phổ nhạc"><input class="input" value=${form.arranger} onInput=${set("arranger")} /></${Field}></div>
      </div>
      <${Field} label=${`Lời bài hát${lines ? ` · ${lines} dòng` : ""}`} hint="Dán nguyên lời. Bước 1 sẽ soát chính tả, địa danh và chia đoạn.">
        <textarea class="textarea" rows="12" placeholder="Cảm ơn mẹ đã yêu cha…" value=${form.lyrics} onInput=${set("lyrics")}></textarea>
      </${Field}>
    </div>
  </${Modal}>`;
}
