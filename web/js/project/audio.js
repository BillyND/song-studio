// Step 3: add the song made in Suno (drop, Downloads or path).
import { ago, api, bumpRuns, bytes, html, media, render, t, toast, toastError, useEffect, useState } from "../core.js";
import { Icon } from "../icons.js";
import { Btn } from "../ui.js";
import { stepDone } from "./actions.js";
import { VideoCTA } from "./parts.js";

export function AudioStep({ d, slug, reload, stamp }) {
  const [recent, setRecent] = useState([]);
  const [over, setOver] = useState(false);
  const [busy, setBusy] = useState(false);
  const [path, setPath] = useState("");
  useEffect(() => { api("/api/downloads/audio").then(setRecent).catch(() => {}); }, []);
  const importPath = async (p) => {
    setBusy(true);
    try { await api(`/api/projects/${encodeURIComponent(slug)}/audio`, { method: "POST", body: { path: p } }); toast(t("Đã thêm nhạc")); bumpRuns(); await reload(); }
    catch (e) { toastError(e); } finally { setBusy(false); }
  };
  const upload = async (file) => {
    const fd = new FormData();
    fd.append("file", file);
    setBusy(true);
    try { await api(`/api/projects/${encodeURIComponent(slug)}/audio`, { method: "POST", body: fd }); toast(t("Đã tải lên")); await reload(); }
    catch (e) { toastError(e); } finally { setBusy(false); }
  };
  return html`<div class="stack">
    ${d.audio && html`<div><div class="row" style="margin-bottom:8px"><${Icon} name="music" /><b>${d.audio}</b></div><audio controls preload="metadata" src=${media(slug, d.audio, stamp)}></audio></div>`}
    ${d.audio && !stepDone(d, "render") && html`<${VideoCTA} d=${d} slug=${slug} />`}
    ${d.audio && html`<div class="muted small">${t("Muốn đổi bản nhạc khác? Kéo file mới vào bên dưới.")}</div>`}
    <div class=${"dropzone" + (over ? " over" : "")} onDragOver=${(e) => { e.preventDefault(); setOver(true); }} onDragLeave=${() => setOver(false)}
      onDrop=${(e) => { e.preventDefault(); setOver(false); const f = e.dataTransfer.files[0]; f && upload(f); }}>
      <${Icon} name="upload" size=${24} /><div style="margin:8px 0 4px;font-weight:600">${t("Kéo file M4A từ Suno vào đây")}</div>
      <div class="muted small">${t("hoặc")} <label style="color:var(--accent-text);cursor:pointer">${t("chọn file")}<input type="file" accept="audio/*,.m4a" hidden onChange=${(e) => e.target.files[0] && upload(e.target.files[0])} /></label></div>
      ${busy && html`<div style="margin-top:8px"><${Icon} name="loader" class="spin" /></div>`}
    </div>
    ${recent.length > 0 && html`<div><h3 style="margin-bottom:8px">${t("Mới tải trong Downloads")}</h3><div class="file-list">
      ${recent.map((f) => html`<div class="file-item" onClick=${() => importPath(f.path)}><${Icon} name="music" /><div class="grow"><div style="font-weight:600">${f.name}</div>
        <div class="muted small">${bytes(f.size)} · ${ago(f.modified)}</div></div><span class="pill accent">${t("Dùng file này")}</span></div>`)}</div></div>`}
    <div class="row"><input class="input" placeholder=${t("hoặc dán đường dẫn file, ví dụ ~/Downloads/Bai hat.m4a")} value=${path} onInput=${(e) => setPath(e.target.value)} />
      <${Btn} disabled=${!path.trim()} busy=${busy} onClick=${() => importPath(path)}>${t("Thêm")}</${Btn}></div>
  </div>`;
}
