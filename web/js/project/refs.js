// Reference songs: add by drop, Downloads or path, and show their analysis.
import { ago, api, bytes, html, isVideo, media, t, toast, toastError, useEffect, useState } from "../core.js";
import { Icon } from "../icons.js";
import { Btn } from "../ui.js";
import { runAction } from "./actions.js";
import { Stat } from "./parts.js";

// Pull the headline numbers out of analyze-reference.py's text report.
function parseAnalysis(text) {
  if (!text) return null;
  const m = (re) => (text.match(re) || [])[1];
  const hp = parseFloat(m(/Harmonic\/percussive energy ([\d.]+)/));
  const bass = parseFloat(m(/Bass \(<150 Hz\) share overall (\d+)%/));
  const loud = text.split("\n").filter((l) => /^\d+:\d/.test(l));
  return {
    duration: m(/Duration ([\d:.]+)/),
    tempo: m(/Tempo ([\d.]+) BPM/), range: m(/local range ([\d-]+)/),
    key: m(/Key candidates: ([^(,]+)/), chords: (m(/Most common chords \(beats\): (.*)/) || "").split(", ").slice(0, 6).map((c) => c.split(":")[0]).join(" · "),
    meter: (() => { const a = parseFloat(m(/3\/4=([\d.]+)/)), b = parseFloat(m(/4\/4=([\d.]+)/)); return isFinite(a) && isFinite(b) ? (Math.abs(a - b) < 0.03 ? t("chưa rõ") : a > b ? "3/4" : "4/4") : ""; })(),
    perc: isFinite(hp) ? (hp >= 9 ? t("rất nhẹ ({v})", { v: hp }) : hp >= 6 ? t("nhẹ ({v})", { v: hp }) : t("rõ ({v})", { v: hp })) : "",
    bass: isFinite(bass) ? (bass < 12 ? t("{n}% · mỏng", { n: bass }) : bass > 30 ? t("{n}% · dày", { n: bass }) : `${bass}%`) : "",
    sections: loud.length,
  };
}


export function RefsPanel({ d, slug, reload, stamp }) {
  const [recent, setRecent] = useState([]);
  const [over, setOver] = useState(false);
  const [busy, setBusy] = useState(false);
  const [path, setPath] = useState("");
  useEffect(() => { api("/api/downloads/audio?kind=ref").then(setRecent).catch(() => {}); }, []);
  const base = `/api/projects/${encodeURIComponent(slug)}/refs`;
  const add = async (body) => {
    setBusy(true);
    try { await api(base, { method: "POST", body }); toast(t("Đã thêm bài mẫu")); setPath(""); await reload(); }
    catch (e) { toastError(e); } finally { setBusy(false); }
  };
  const upload = (file) => { const fd = new FormData(); fd.append("file", file); add(fd); };
  const remove = async (name) => {
    if (!confirm(t("Bỏ bài mẫu {name}?", { name }))) return;
    try { await api(`${base}/${encodeURIComponent(name)}`, { method: "DELETE" }); await reload(); } catch (e) { toastError(e); }
  };
  const have = new Set(d.refs.map((r) => r.name));
  return html`<div class="stack" style="gap:10px">
    <div class=${"dropzone compact" + (over ? " over" : "")} onDragOver=${(e) => { e.preventDefault(); setOver(true); }} onDragLeave=${() => setOver(false)}
      onDrop=${(e) => { e.preventDefault(); setOver(false); [...e.dataTransfer.files].forEach(upload); }}>
      <${Icon} name="upload" size=${18} /><span><b>${t("Kéo bài mẫu vào đây")}</b> <span class="muted">${t("· mp3, m4a, mp4, webm… hoặc")}</span> <label style="color:var(--accent-text);cursor:pointer">${t("chọn file")}<input type="file" accept="audio/*,video/*" multiple hidden onChange=${(e) => [...e.target.files].forEach(upload)} /></label></span>
      ${busy && html`<${Icon} name="loader" class="spin" />`}
    </div>
    ${recent.length > 0 && html`<details><summary class="small muted" style="cursor:pointer">${t("Chọn từ Downloads")}</summary>
      <div class="file-list" style="margin-top:8px">${recent.map((f) => html`<div class="file-item" onClick=${() => add({ path: f.path })}><${Icon} name=${isVideo(f.name) ? "film" : "music"} />
        <div class="grow" style="min-width:0"><div class="ellipsis" style="font-weight:600">${f.name}</div><div class="muted small">${bytes(f.size)} · ${ago(f.modified)}</div></div><span class="pill accent">${t("Thêm")}</span></div>`)}</div></details>`}
    <div class="row wrap"><input class="input grow" placeholder=${t("hoặc dán đường dẫn file trong máy")} value=${path} onInput=${(e) => setPath(e.target.value)} />
      <${Btn} disabled=${!path.trim()} busy=${busy} onClick=${() => add({ path })}>${t("Thêm")}</${Btn}></div>
    ${d.refs.map((r) => { const a = parseAnalysis(r.analysis); return html`<div class="ref-card">
      <div class="row"><${Icon} name=${isVideo(r.name) ? "film" : "music"} /><b class="grow ellipsis">${r.name}</b><span class="muted small">${bytes(r.size)}</span>
        <${Btn} size="sm" kind="ghost" icon="x" title=${t("Bỏ bài mẫu")} onClick=${() => remove(r.name)} /></div>
      ${isVideo(r.name) ? html`<video controls preload="metadata" style="max-height:240px" src=${media(slug, r.path, stamp)}></video>` : html`<audio controls preload="metadata" src=${media(slug, r.path, stamp)}></audio>`}
      ${a ? html`<div class="analysis">
          <${Stat} k="Tempo" v=${a.tempo && a.tempo + " BPM"} s=${a.range && t("dao động {range}", { range: a.range })} />
          <${Stat} k=${t("Giọng")} v=${a.key} s=${a.chords} />
          <${Stat} k=${t("Nhịp")} v=${a.meter} />
          <${Stat} k=${t("Bộ gõ")} v=${a.perc} />
          <${Stat} k="Bass" v=${a.bass} />
          <${Stat} k=${t("Dài")} v=${a.duration} s=${a.sections ? t("{n} đoạn", { n: a.sections }) : ""} />
        </div>
        <details><summary class="small muted" style="cursor:pointer">${t("Số đo đầy đủ")}</summary><pre class="block" style="margin-top:8px">${r.analysis}</pre></details>`
        : html`<div class="muted small">${t("Chưa phân tích, sẽ tự phân tích khi bấm Tạo prompt Suno.")}</div>`}
    </div>`; })}
    ${d.refs.some((r) => !r.analysis) && html`<div><${Btn} size="sm" icon="eye" onClick=${() => runAction(slug, "ref.analyze")}>${t("Chỉ phân tích bài mẫu")}</${Btn}></div>`}
  </div>`;
}
