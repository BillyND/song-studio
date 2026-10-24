// Song list and the "new song" dialog.
import { html, useState, useEffect, api, toast, toastError, go, ago, bytes, media, useStore, openRun, bumpRuns } from "./core.js";
import { Icon } from "./icons.js";
import { Btn, Modal, Field, Empty, PresetChips, Switch, GenrePicker } from "./ui.js";

export function ProjectsPage() {
  const [list, setList] = useState(null);
  const [creating, setCreating] = useState(false);
  const runsVersion = useStore((s) => s.runsVersion);
  useEffect(() => { api("/api/projects").then(setList).catch(toastError); }, [runsVersion]);
  return html`<div class="page">
    <div class="page-head">
      <div><div class="eyebrow">Song Studio</div><h1>Bài hát</h1>
        <div class="muted" style="margin-top:6px">Mỗi bài, thuộc thể loại nào cũng vậy, đi qua 9 bước: từ lời tới bài hát, video karaoke và bản nhạc. Bấm vào một bài để làm tiếp.</div></div>
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

const isVideo = (n) => /\.(mp4|webm|mov|mkv)$/i.test(n);

function NewSong({ onClose }) {
  const boot = useStore((s) => s.boot);
  const defaults = boot?.config?.credits || [["Sáng tác: ", ""], ["Phổ nhạc: ", ""]];
  const [form, setForm] = useState({ title: "", subtitle: "", lyrics: "", brief: "", genre: "", language: "vi" });
  const [credits, setCredits] = useState(defaults.map((c) => [...c]));
  const [autoPrompt, setAutoPrompt] = useState(true);
  // reference songs: picked Downloads paths and dropped/browsed File objects
  const [refs, setRefs] = useState([]);
  const [recent, setRecent] = useState([]);
  const [analyze, setAnalyze] = useState(true);
  const [over, setOver] = useState(false);
  const [busy, setBusy] = useState("");
  useEffect(() => { api("/api/downloads/audio?kind=ref").then(setRecent).catch(() => {}); }, []);
  const set = (k) => (e) => { const v = e.target.value; setForm((f) => ({ ...f, [k]: v })); };
  const addFiles = (files) => setRefs((r) => [...r, ...[...files].map((f) => ({ name: f.name, size: f.size, file: f }))]);
  const addPath = (f) => setRefs((r) => (r.some((x) => x.path === f.path) ? r : [...r, { name: f.name, size: f.size, path: f.path }]));

  const submit = async () => {
    setBusy("Đang tạo bài…");
    let r;
    try {
      r = await api("/api/projects", { method: "POST", body: { title: form.title, subtitle: form.subtitle, lyrics: form.lyrics, credits, genre: form.genre, language: form.language } });
    } catch (e) { toastError(e); setBusy(""); return; }
    try {
      const base = `/api/projects/${encodeURIComponent(r.slug)}`;
      for (const [i, ref] of refs.entries()) {
        setBusy(`Đang chép bài mẫu ${i + 1}/${refs.length}…`);
        if (ref.file) { const fd = new FormData(); fd.append("file", ref.file); await api(base + "/refs", { method: "POST", body: fd }); }
        else await api(base + "/refs", { method: "POST", body: { path: ref.path } });
      }
      if (form.brief.trim()) await api(base + "/files", { method: "PUT", body: { path: "brief.md", content: form.brief.trim() + "\n" } });
      if (autoPrompt) {
        setBusy("Đang xếp việc cho agent…");
        const metas = await api(base + "/pipeline/suno", { method: "POST", body: {} });
        openRun((metas.find((m) => !(m.after || []).length) || metas[0]).id);
        toast(`Đã xếp ${metas.length} việc: ${refs.length ? "phân tích bài mẫu ‖ soát lời → phối khí" : "soát lời → phối khí"}`);
      } else if (refs.length && analyze) {
        const meta = await api(base + "/actions/ref.analyze", { method: "POST", body: {} });
        openRun(meta.id);
      }
      bumpRuns();
      go("/p/" + encodeURIComponent(r.slug) + (autoPrompt ? "/suno" : ""));
    } catch (e) {
      // the song already exists: open it so a retry does not create a duplicate
      toastError(new Error("Đã tạo bài nhưng bước sau lỗi: " + e.message + ". Làm tiếp trong trang bài."));
      bumpRuns();
      go("/p/" + encodeURIComponent(r.slug) + "/lyrics");
    }
  };
  const lines = form.lyrics.trim() ? form.lyrics.trim().split("\n").length : 0;
  return html`<${Modal} title="Bài hát mới" onClose=${onClose} footer=${html`
      ${busy && html`<span class="muted small grow">${busy}</span>`}
      <${Btn} kind="ghost" onClick=${onClose}>Huỷ</${Btn}>
      <${Btn} kind="primary" icon="sparkles" busy=${!!busy} disabled=${!form.title.trim() || !form.lyrics.trim()} onClick=${submit}>Tạo bài</${Btn}>`}>
    <div class="stack">
      <div class="form-row">
        <div class="grow"><${Field} label="Tên bài (dòng lớn trên ảnh tựa đề)"><input class="input" placeholder="Tên bài" value=${form.title} onInput=${set("title")} autofocus /></${Field}></div>
        <div class="grow"><${Field} label="Dòng phụ"><input class="input" placeholder="không bắt buộc" value=${form.subtitle} onInput=${set("subtitle")} /></${Field}></div>
      </div>
      <${GenrePicker} genre=${form.genre} language=${form.language} onChange=${(v) => setForm((f) => ({ ...f, ...v }))} />
      <div class="form-row">${credits.map((c, i) => html`
        <div class="grow"><${Field} label=${c[0].replace(/:\s*$/, "")}><input class="input" value=${c[1]} onInput=${(e) => { const v = e.target.value; setCredits((cs) => cs.map((x, j) => (j === i ? [x[0], v] : x))); }} /></${Field}></div>`)}
      </div>
      <${Field} label=${`Lời bài hát${lines ? ` · ${lines} dòng` : ""}`} hint="Dán nguyên lời. Bước 1 sẽ soát chính tả, địa danh và chia đoạn.">
        <textarea class="textarea" rows="9" placeholder="Dán lời bài hát…" value=${form.lyrics} onInput=${set("lyrics")}></textarea>
      </${Field}>

      <div class="divider"></div>
      <div><div class="row"><${Icon} name="wand" /><h3>Bài mẫu & yêu cầu</h3><span class="muted small">không bắt buộc</span></div>
        <div class="muted small" style="margin-top:4px">Bài mẫu chỉ để agent học phong cách (tiết tấu, dàn nhạc, màu giọng), được phân tích trên máy và không gửi lên Suno.</div></div>
      <div class=${"dropzone" + (over ? " over" : "")} style="padding:16px" onDragOver=${(e) => { e.preventDefault(); setOver(true); }} onDragLeave=${() => setOver(false)}
        onDrop=${(e) => { e.preventDefault(); setOver(false); addFiles(e.dataTransfer.files); }}>
        <${Icon} name="upload" size=${20} /><div style="margin:6px 0 2px;font-weight:600">Kéo bài mẫu vào đây</div>
        <div class="muted small">mp3, m4a, mp4, webm… hoặc <label style="color:var(--accent);cursor:pointer">chọn file<input type="file" accept="audio/*,video/*" multiple hidden onChange=${(e) => addFiles(e.target.files)} /></label></div>
      </div>
      ${recent.length > 0 && html`<div class="file-list">${recent.slice(0, 5).map((f) => { const on = refs.some((r) => r.path === f.path); return html`
        <div class="file-item" style=${on ? { borderColor: "var(--accent)" } : {}} onClick=${() => (on ? setRefs(refs.filter((r) => r.path !== f.path)) : addPath(f))}>
          <${Icon} name=${isVideo(f.name) ? "film" : "music"} /><div class="grow" style="min-width:0"><div style="font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${f.name}</div>
          <div class="muted small">Downloads · ${bytes(f.size)} · ${ago(f.modified)}</div></div>
          <span class=${"pill " + (on ? "ok" : "accent")}>${on ? "Đã chọn" : "Chọn"}</span></div>`; })}</div>`}
      ${refs.length > 0 && html`<div class="stack" style="gap:6px">${refs.map((r, i) => html`<div class="row small">
        <${Icon} name="check" size=${14} /><span class="grow" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${r.name}</span><span class="muted">${bytes(r.size)}</span>
        <${Btn} size="sm" kind="ghost" icon="x" title="Bỏ" onClick=${() => setRefs(refs.filter((_, j) => j !== i))} /></div>`)}
        ${!autoPrompt && html`<${Switch} on=${analyze} onChange=${setAnalyze}>Phân tích bài mẫu ngay sau khi tạo</${Switch}>`}</div>`}
      <${Field} label="Yêu cầu mở rộng" hint="Ví dụ lời dặn của người đặt bài. Agent dùng khi tạo kiểu phối riêng ở bước 0.">
        <textarea class="textarea" rows="3" style="min-height:70px" placeholder="Ví dụ: nhịp vừa, giọng nữ ấm, điệp khúc bùng nổ…" value=${form.brief} onInput=${set("brief")}></textarea>
      </${Field}>
      <${PresetChips} genre=${form.genre} value=${form.brief} onChange=${(v) => setForm((f) => ({ ...f, brief: v }))} />
      <div class="divider"></div>
      <${Switch} on=${autoPrompt} onChange=${setAutoPrompt}>Tạo prompt Suno ngay: agent ${boot?.config?.defaultAgent || "claude"} soát lời song song với phân tích bài mẫu, rồi phối khí theo yêu cầu</${Switch}>
    </div>
  </${Modal}>`;
}
