// One song's workspace: the eight-step workflow with script and agent actions.
import { html, useState, useEffect, useRef, api, toast, toastError, openRun, bumpRuns, go, ago, secs, bytes, media, copyText, useStore, runDuration } from "./core.js";
import { Icon } from "./icons.js";
import { Btn, StatusPill, Seg, Switch, Modal, Lightbox, Markdown, Empty, AgentPicker, useAgentChoice, AgentIcon, Field, PresetChips } from "./ui.js";

const STEP_ICON = { lyrics: "type", suno: "sparkles", audio: "music", align: "mic", trim: "scissors", scenes: "image", stills: "eye", render: "film", sheet: "sheet" };

export function ProjectPage({ slug, step }) {
  const [d, setD] = useState(null);
  const [err, setErr] = useState(null);
  const [stamp, setStamp] = useState(Date.now());
  const runsVersion = useStore((s) => s.runsVersion);
  const load = async () => {
    try { setD(await api(`/api/projects/${encodeURIComponent(slug)}`)); setStamp(Date.now()); setErr(null); }
    catch (e) { setErr(e); }
  };
  useEffect(() => { load(); }, [slug, runsVersion]);
  const running = d?.steps.some((s) => s.status === "running");
  useEffect(() => {
    if (!running) return;
    const t = setInterval(load, 2500);
    return () => clearInterval(t);
  }, [running, slug]);

  if (err) return html`<div class="page"><${Empty} icon="alert" title="Không mở được bài">${err.message}</${Empty}></div>`;
  if (!d) return html`<div class="page"><div class="empty"><${Icon} name="loader" class="spin" size=${22} /></div></div>`;
  const current = step || (d.steps.find((s) => s.status === "todo" || s.status === "stale") || d.steps[d.steps.length - 1]).id;
  const ctx = { d, slug, reload: load, stamp };
  const S = { lyrics: LyricsStep, suno: SunoStep, audio: AudioStep, align: AlignStep, trim: TrimStep, scenes: ScenesStep, stills: StillsStep, render: RenderStep, sheet: SheetStep }[current] || LyricsStep;
  const stepMeta = d.steps.find((s) => s.id === current);
  return html`<div class="page">
    <div class="hero">
      <div class=${"hero-bg" + (d.cover ? "" : " none")} style=${d.cover ? { backgroundImage: `url("${media(slug, d.cover, stamp)}")` } : {}}></div>
      <div class="hero-in">
        <div class="grow">
          <a class="small" style="color:rgba(255,255,255,.75)" href="#/">← Bài hát</a>
          <h1>${d.title} <span style="font-weight:500;opacity:.8">${d.subtitle}</span></h1>
          <div class="muted small">${d.done}/${d.total} bước xong · cập nhật ${ago(d.updated)}<span class="hero-path"> · <span class="mono">${d.dir}</span></span></div>
        </div>
        <div class="hero-actions">
          <${Btn} size="sm" icon="folder" onClick=${() => api(`/api/projects/${slug}/open`, { method: "POST", body: { path: "" } }).catch(toastError)}>Mở thư mục</${Btn}>
          <${FreeAgentButton} slug=${slug} />
          <${Btn} size="sm" icon="trash" title="Xoá bài" onClick=${() => trashProject(d, slug)} />
          <${PrimaryAction} d=${d} slug=${slug} />
        </div>
      </div>
    </div>
    <div class="workspace">
      <div class="card steps">${d.steps.map((s, i) => html`
        <div class=${`step ${s.status}${s.id === current ? " active" : ""}`} onClick=${() => go(`/p/${encodeURIComponent(slug)}/${s.id}`)}>
          <div class="step-dot">${s.status === "done" || s.status === "skip" ? html`<${Icon} name="check" size=${15} />` : s.status === "running" ? html`<${Icon} name="loader" size=${15} />` : s.status === "stale" ? html`<${Icon} name="refresh" size=${14} />` : i + 1}</div>
          <div style="min-width:0"><div class="step-title">${s.title}</div><div class="step-detail" title=${s.detail}>${s.detail}</div></div>
        </div>`)}</div>
      <div class="stack">
        <div class="card card-pad">
          <div class="panel-title"><${Icon} name=${STEP_ICON[current]} size=${20} /><h2>${stepMeta.title}</h2><${StatusPill} status=${stepMeta.status} /><span class="spacer"></span>
            ${stepMeta.runId && html`<${Btn} size="sm" icon="terminal" onClick=${() => openRun(stepMeta.runId)}>Xem đang chạy</${Btn}>`}</div>
          <div class="muted small" style="margin-bottom:16px">${stepMeta.detail}</div>
          <${S} ...${ctx} />
          <${NextBar} d=${d} slug=${slug} current=${current} />
        </div>
        <${RecentRuns} slug=${slug} step=${current} />
      </div>
    </div>
  </div>`;
}

const stepDone = (d, id) => ["done", "skip"].includes(d.steps.find((s) => s.id === id)?.status);
const goStep = (slug, id) => go(`/p/${encodeURIComponent(slug)}/${id}`);

// PrimaryAction is the single "do the next thing" button in the song header.
function PrimaryAction({ d, slug }) {
  const running = d.steps.some((s) => s.status === "running");
  if (running) return html`<${Btn} size="sm" kind="primary" icon="loader" onClick=${() => openRun(d.steps.find((s) => s.status === "running").runId)}>Đang chạy…</${Btn}>`;
  if (!d.audio && !stepDone(d, "suno")) return html`<${Btn} size="sm" kind="primary" icon="sparkles" onClick=${() => goStep(slug, "lyrics")}>Tạo prompt Suno</${Btn}>`;
  if (!d.audio) return html`<${Btn} size="sm" kind="primary" icon="music" onClick=${() => goStep(slug, "audio")}>Thêm nhạc từ Suno</${Btn}>`;
  if (!stepDone(d, "render")) return html`<${Btn} size="sm" kind="primary" icon="film" onClick=${async () => { if (await runPipeline(slug, "video")) goStep(slug, "render"); }}>Làm video</${Btn}>`;
  return html`<${Btn} size="sm" kind="primary" icon="play" onClick=${() => goStep(slug, "render")}>Xem video</${Btn}>`;
}

// VideoCTA explains and starts the whole video chain in one click.
function VideoCTA({ d, slug }) {
  const left = ["align", "trim", "scenes", "stills", "render", "sheet"].filter((id) => !stepDone(d, id)).length;
  return html`<div class="cta">
    <div><div class="row" style="gap:8px"><${Icon} name="film" /><b>Làm video karaoke từ bài này</b></div>
      <div class="muted small" style="margin-top:4px">Studio tự canh lời theo giọng hát → cắt bớt nhạc dạo nếu dài → agent lên danh sách cảnh → tạo ảnh AI → render khung xem trước → xuất video 1080p, kèm bản nhạc có lời. Còn ${left} bước, khoảng 15–25 phút; bạn có thể đóng trang, việc vẫn chạy.</div></div>
    <div class="cta-go"><${Btn} kind="primary" icon="zap" busy=${d.steps.some((s) => s.status === "running")} onClick=${async () => { if (await runPipeline(slug, "video")) goStep(slug, "render"); }}>Làm video tự động</${Btn}></div>
  </div>`;
}

// NextBar sits under every step and says what comes next.
function NextBar({ d, slug, current }) {
  const i = d.steps.findIndex((s) => s.id === current);
  const next = d.steps[i + 1];
  if (!next) return null;
  const here = d.steps[i];
  const ready = here.status === "done" || here.status === "skip";
  return html`<div class="next-bar">
    <span class="muted small">${ready ? "Bước này đã xong." : "Làm xong bước này rồi sang:"}</span>
    <span class="spacer"></span>
    <${Btn} kind=${ready ? "primary" : ""} icon="chevron" onClick=${() => goStep(slug, next.id)}>Tiếp: ${next.title}</${Btn}>
  </div>`;
}

async function runAction(slug, action, body = {}) {
  try {
    const meta = await api(`/api/projects/${encodeURIComponent(slug)}/actions/${action}`, { method: "POST", body });
    openRun(meta.id);
    bumpRuns();
    toast(meta.status === "queued" ? "Đã xếp hàng, sẽ chạy khi tới lượt" : "Đã bắt đầu");
    return meta;
  } catch (e) { toastError(e); }
}

const pending = new Set(); // chains being submitted, so a double click queues nothing twice

async function runPipeline(slug, name, body = {}) {
  const key = slug + "/" + name;
  if (pending.has(key)) return;
  pending.add(key);
  try {
    const metas = await api(`/api/projects/${encodeURIComponent(slug)}/pipeline/${name}`, { method: "POST", body });
    openRun((metas.find((m) => !(m.after || []).length) || metas[0]).id);
    bumpRuns();
    toast(`Đã xếp ${metas.length} việc, các việc độc lập chạy song song`);
    return metas;
  } catch (e) { toastError(e); } finally { pending.delete(key); }
}

// ChainStatus shows the newest chain of runs for these steps, e.g. analysis ‖ lyrics → arrangement.
function ChainStatus({ slug, steps, title }) {
  const [runs, setRuns] = useState([]);
  const runsVersion = useStore((s) => s.runsVersion);
  const load = () => api(`/api/runs?project=${encodeURIComponent(slug)}&limit=60`).then(setRuns).catch(() => {});
  useEffect(() => { load(); }, [slug, runsVersion]);
  const group = runs.find((r) => r.group && steps.includes(r.step))?.group;
  const chain = group ? runs.filter((r) => r.group === group).reverse() : [];
  const live = chain.some((r) => r.status === "running" || r.status === "queued");
  useEffect(() => { if (!live) return; const t = setInterval(load, 1500); return () => clearInterval(t); }, [live, group]);
  if (!chain.length) return null;
  return html`<div class="chain">
    <span class="muted small" style="font-weight:650">${title}</span>
    ${chain.map((r, i) => html`${i > 0 && html`<${Icon} name="chevron" size=${12} class="muted" />`}
      <button type="button" class="chain-step" onClick=${() => openRun(r.id)} title=${r.title}>
        <${AgentIcon} agent=${r.kind === "agent" ? r.agent : "script"} /><span>${r.title}</span><${StatusPill} status=${r.status} /></button>`)}
  </div>`;
}

function ActionCard({ title, hint, children, onRun, label, busy, icon = "play", agent }) {
  return html`<div class="action-bar">
    <div class="grow" style="min-width:220px"><div class="row" style="gap:8px">${agent ? html`<${Icon} name="bot" size=${15} />` : html`<${Icon} name="terminal" size=${15} />`}<b>${title}</b></div>
      ${hint && html`<div class="muted small">${hint}</div>`}</div>
    ${children}
    <${Btn} kind="primary" icon=${icon} busy=${busy} onClick=${onRun}>${label || "Chạy"}</${Btn}>
  </div>`;
}

function AgentAction({ slug, action, title, hint, extra, body = {} }) {
  const ag = useAgentChoice();
  const [note, setNote] = useState("");
  return html`<div class="stack" style="gap:10px">
    <${ActionCard} agent title=${title} hint=${hint} label="Giao cho agent" icon="send"
      onRun=${() => runAction(slug, action, { agent: ag.agent, model: ag.model, note, ...body })}>
      <${AgentPicker} value=${ag.agent} onChange=${ag.setAgent} model=${ag.model} onModel=${ag.setModel} />
    </${ActionCard}>
    ${extra}
    <textarea class="textarea" rows="2" style="min-height:44px" placeholder="Dặn thêm agent (không bắt buộc)…" value=${note} onInput=${(e) => setNote(e.target.value)}></textarea>
    <${PresetChips} value=${note} onChange=${setNote} />
  </div>`;
}

// useSave returns [busy, save]; save resolves true only when the file was written.
function useSave(slug, reload) {
  const [busy, setBusy] = useState(false);
  return [busy, async (path, content, sha) => {
    setBusy(true);
    try { await api(`/api/projects/${encodeURIComponent(slug)}/files`, { method: "PUT", body: { path, content, sha } }); toast("Đã lưu"); await reload(); return true; }
    catch (e) { toastError(e.status === 409 ? new Error("project.json vừa được sửa ở nơi khác (có thể bởi agent). Đóng và mở lại để lấy bản mới.") : e); return false; }
    finally { setBusy(false); }
  }];
}

// ---------- steps ----------

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
    meter: (() => { const a = parseFloat(m(/3\/4=([\d.]+)/)), b = parseFloat(m(/4\/4=([\d.]+)/)); return isFinite(a) && isFinite(b) ? (Math.abs(a - b) < 0.03 ? "chưa rõ" : a > b ? "3/4" : "4/4") : ""; })(),
    perc: isFinite(hp) ? (hp >= 9 ? `rất nhẹ (${hp})` : hp >= 6 ? `nhẹ (${hp})` : `rõ (${hp})`) : "",
    bass: isFinite(bass) ? `${bass}%${bass < 12 ? " · mỏng" : bass > 30 ? " · dày" : ""}` : "",
    sections: loud.length,
  };
}

function Stat({ k, v, s }) {
  return v ? html`<div class="stat"><div class="k">${k}</div><div class="v">${v}</div>${s && html`<div class="s">${s}</div>`}</div>` : null;
}

const isVideo = (n) => /\.(mp4|webm|mov|mkv)$/i.test(n);

function RefsPanel({ d, slug, reload, stamp }) {
  const [recent, setRecent] = useState([]);
  const [over, setOver] = useState(false);
  const [busy, setBusy] = useState(false);
  const [path, setPath] = useState("");
  useEffect(() => { api("/api/downloads/audio?kind=ref").then(setRecent).catch(() => {}); }, []);
  const base = `/api/projects/${encodeURIComponent(slug)}/refs`;
  const add = async (body) => {
    setBusy(true);
    try { await api(base, { method: "POST", body }); toast("Đã thêm bài mẫu"); setPath(""); await reload(); }
    catch (e) { toastError(e); } finally { setBusy(false); }
  };
  const upload = (file) => { const fd = new FormData(); fd.append("file", file); add(fd); };
  const remove = async (name) => {
    if (!confirm(`Bỏ bài mẫu ${name}?`)) return;
    try { await api(`${base}/${encodeURIComponent(name)}`, { method: "DELETE" }); await reload(); } catch (e) { toastError(e); }
  };
  const have = new Set(d.refs.map((r) => r.name));
  return html`<div class="stack" style="gap:10px">
    <div class=${"dropzone compact" + (over ? " over" : "")} onDragOver=${(e) => { e.preventDefault(); setOver(true); }} onDragLeave=${() => setOver(false)}
      onDrop=${(e) => { e.preventDefault(); setOver(false); [...e.dataTransfer.files].forEach(upload); }}>
      <${Icon} name="upload" size=${18} /><span><b>Kéo bài mẫu vào đây</b> <span class="muted">· mp3, m4a, mp4, webm… hoặc</span> <label style="color:var(--accent);cursor:pointer">chọn file<input type="file" accept="audio/*,video/*" multiple hidden onChange=${(e) => [...e.target.files].forEach(upload)} /></label></span>
      ${busy && html`<${Icon} name="loader" class="spin" />`}
    </div>
    ${recent.length > 0 && html`<details><summary class="small muted" style="cursor:pointer">Chọn từ Downloads</summary>
      <div class="file-list" style="margin-top:8px">${recent.map((f) => html`<div class="file-item" onClick=${() => add({ path: f.path })}><${Icon} name=${isVideo(f.name) ? "film" : "music"} />
        <div class="grow" style="min-width:0"><div class="ellipsis" style="font-weight:600">${f.name}</div><div class="muted small">${bytes(f.size)} · ${ago(f.modified)}</div></div><span class="pill accent">Thêm</span></div>`)}</div></details>`}
    <div class="row wrap"><input class="input grow" placeholder="hoặc dán đường dẫn file trong máy" value=${path} onInput=${(e) => setPath(e.target.value)} />
      <${Btn} disabled=${!path.trim()} busy=${busy} onClick=${() => add({ path })}>Thêm</${Btn}></div>
    ${d.refs.map((r) => { const a = parseAnalysis(r.analysis); return html`<div class="ref-card">
      <div class="row"><${Icon} name=${isVideo(r.name) ? "film" : "music"} /><b class="grow ellipsis">${r.name}</b><span class="muted small">${bytes(r.size)}</span>
        <${Btn} size="sm" kind="ghost" icon="x" title="Bỏ bài mẫu" onClick=${() => remove(r.name)} /></div>
      ${isVideo(r.name) ? html`<video controls preload="metadata" style="max-height:240px" src=${media(slug, r.path, stamp)}></video>` : html`<audio controls preload="metadata" src=${media(slug, r.path, stamp)}></audio>`}
      ${a ? html`<div class="analysis">
          <${Stat} k="Tempo" v=${a.tempo && a.tempo + " BPM"} s=${a.range && "dao động " + a.range} />
          <${Stat} k="Giọng" v=${a.key} s=${a.chords} />
          <${Stat} k="Nhịp" v=${a.meter} />
          <${Stat} k="Bộ gõ" v=${a.perc} />
          <${Stat} k="Bass" v=${a.bass} />
          <${Stat} k="Dài" v=${a.duration} s=${a.sections ? a.sections + " đoạn" : ""} />
        </div>
        <details><summary class="small muted" style="cursor:pointer">Số đo đầy đủ</summary><pre class="block" style="margin-top:8px">${r.analysis}</pre></details>`
        : html`<div class="muted small">Chưa phân tích, sẽ tự phân tích khi bấm Tạo prompt Suno.</div>`}
    </div>`; })}
    ${d.refs.some((r) => !r.analysis) && html`<div><${Btn} size="sm" icon="eye" onClick=${() => runAction(slug, "ref.analyze")}>Chỉ phân tích bài mẫu</${Btn}></div>`}
  </div>`;
}

function LyricsStep({ d, slug, reload, stamp }) {
  const [text, setText] = useState(d.lyrics);
  const [brief, setBrief] = useState(d.brief);
  const [check, setCheck] = useState(null);
  const [fixAll, setFixAll] = useState(false);
  const ag = useAgentChoice();
  const [busy, save] = useSave(slug, reload);
  useEffect(() => setText(d.lyrics), [d.lyrics]);
  useEffect(() => setBrief(d.brief), [d.brief]);
  const dirty = text !== d.lyrics || brief !== d.brief;
  const saveAll = async () => {
    if (text !== d.lyrics && !(await save("lyrics.txt", text))) return false;
    if (brief !== d.brief && !(await save("brief.md", brief))) return false;
    return true;
  };
  const [starting, setStarting] = useState(false);
  const runCheck = async () => {
    if (text !== d.lyrics) await save("lyrics.txt", text);
    try { setCheck(await api(`/api/projects/${encodeURIComponent(slug)}/lucbat`)); } catch (e) { toastError(e); }
  };
  const generate = async () => {
    setStarting(true);
    try {
      if (!(await saveAll())) return; // never build a prompt from text that failed to save
      const metas = await runPipeline(slug, "suno", { agent: ag.agent, model: ag.model, fixAll });
      if (metas) go(`/p/${encodeURIComponent(slug)}/suno`);
    } finally { setStarting(false); }
  };
  const chainRunning = d.steps.some((s) => (s.id === "suno" || s.id === "lyrics") && s.status === "running");
  const issues = check ? check.filter((r) => r.notes.some((n) => !n.startsWith("có tiếng đệm"))).length : 0;
  return html`<div class="stack">
    <div class="section-title"><${Icon} name="type" /><h3>Lời bài hát</h3><span class="spacer"></span>
      <${Btn} size="sm" icon="check" onClick=${runCheck}>Kiểm tra lục bát</${Btn}></div>
    <textarea class="textarea" rows="12" value=${text} onInput=${(e) => setText(e.target.value)} onKeyDown=${(e) => (e.metaKey || e.ctrlKey) && e.key === "s" && (e.preventDefault(), saveAll())}></textarea>
    ${check && html`<div class="card flat"><div class="card-head"><h3>Lục bát</h3><span class=${"pill " + (issues ? "warn" : "ok")}>${issues ? `${issues} dòng cần xem` : "Ổn"}</span></div>
      <div class="scroll-x" style="max-height:320px"><table class="table"><thead><tr><th>#</th><th>Lời</th><th>Chữ</th><th>Ghi chú</th></tr></thead><tbody>
      ${check.map((r) => html`<tr><td class="num">${r.line}</td><td>${r.text}</td><td class="num">${r.syllables} · ${r.kind}</td>
        <td class="small">${r.notes.length ? r.notes.map((n) => html`<div class=${n.startsWith("có tiếng đệm") ? "muted" : ""}>${n}</div>`) : html`<span class="pill ok">ok</span>`}</td></tr>`)}
      </tbody></table></div></div>`}

    <div class="section-title"><${Icon} name="music" /><h3>Bài mẫu</h3><span class="muted small">không bắt buộc · chỉ học phong cách, không gửi lên Suno</span></div>
    <${RefsPanel} d=${d} slug=${slug} reload=${reload} stamp=${stamp} />

    <div class="section-title"><${Icon} name="wand" /><h3>Yêu cầu mở rộng</h3><span class="muted small">không bắt buộc</span></div>
    <textarea class="textarea" rows="4" placeholder="Ví dụ: phong cách dân ca mượt mà nhưng không ề à, không dùng bộ gõ…" value=${brief}
      onInput=${(e) => setBrief(e.target.value)} onKeyDown=${(e) => (e.metaKey || e.ctrlKey) && e.key === "s" && (e.preventDefault(), saveAll())}></textarea>
    <${PresetChips} value=${brief} onChange=${setBrief} />

    <div class="cta">
      <div class="cta-fields">
        <label class="field"><span>Agent</span><${AgentPicker} value=${ag.agent} onChange=${ag.setAgent} model=${ag.model} onModel=${ag.setModel} /></label>
        <${Switch} on=${fixAll} onChange=${setFixAll}>Sửa hết đề xuất cấu trúc lời</${Switch}>
      </div>
      <div class="cta-go">
        <div class="muted small">${d.refs.some((r) => !r.analysis) ? "Phân tích bài mẫu ‖ soát lời & chia đoạn → phối khí & Styles" : "Soát lời & chia đoạn → phối khí & Styles"}</div>
        <div class="row wrap">
          ${dirty && html`<${Btn} icon="save" busy=${busy} onClick=${saveAll}>Lưu</${Btn}>`}
          <${Btn} kind="primary" icon="sparkles" busy=${starting} disabled=${!text.trim() || chainRunning} onClick=${generate}>${chainRunning ? "Đang tạo prompt…" : "Tạo prompt Suno"}</${Btn}>
        </div>
      </div>
    </div>
  </div>`;
}

function CopyBlock({ title, text, extra, onSave, saving }) {
  const [edit, setEdit] = useState(false);
  const [val, setVal] = useState(text);
  useEffect(() => setVal(text), [text]);
  return html`<div class="stack" style="gap:8px">
    <div class="row wrap"><h3>${title}</h3>${extra}<span class="spacer"></span>
      ${onSave && html`<${Btn} size="sm" kind="ghost" icon=${edit ? "x" : "code"} onClick=${() => setEdit(!edit)}>${edit ? "Huỷ" : "Sửa"}</${Btn}>`}
      ${edit ? html`<${Btn} size="sm" icon="save" busy=${saving} onClick=${async () => { await onSave(val); setEdit(false); }}>Lưu</${Btn}>`
        : html`<${Btn} size="sm" icon="copy" onClick=${() => copyText(text)}>Chép</${Btn}>`}</div>
    ${edit ? html`<textarea class="textarea mono" rows="10" value=${val} onInput=${(e) => setVal(e.target.value)}></textarea>` : html`<pre class="block">${text}</pre>`}
  </div>`;
}

function SunoStep({ d, slug, reload }) {
  const [busy, save] = useSave(slug, reload);
  const cfg = safeJSON(d.config);
  const n = [...(d.sunoStyle || "").trim()].length;
  const empty = !d.sunoLyrics && !d.sunoStyle;
  return html`<div class="stack">
    <${ChainStatus} slug=${slug} steps=${["lyrics", "suno"]} title="Chuỗi tạo prompt" />
    ${empty ? html`<${Empty} icon="sparkles" title="Chưa có prompt Suno">Ở <a href=${`#/p/${slug}/lyrics`} style="color:var(--accent)">bước 1</a>, thêm lời (và bài mẫu, yêu cầu nếu có) rồi bấm <b>Tạo prompt Suno</b>.</${Empty}>` : html`
      <dl class="kv"><dt>Tên bài</dt><dd><b>${[cfg.title, cfg.subtitle].filter(Boolean).join(" ")}</b></dd><dt>Giọng</dt><dd>${cfg.vocal || "—"}</dd></dl>
      <${CopyBlock} title="Styles" text=${(d.sunoStyle || "").trim()} extra=${html`<span class=${"pill " + (n > 1000 ? "bad" : "ok")}>${n}/1000</span>`} saving=${busy} onSave=${(v) => save("suno/style.txt", v)} />
      <${CopyBlock} title="Lyrics" text=${(d.sunoLyrics || "").trim()} saving=${busy} onSave=${(v) => save("suno/lyrics.txt", v)} />
      ${d.profile && html`<details open><summary class="section-title" style="cursor:pointer"><${Icon} name="wand" /><h3>Vì sao phối như vậy</h3></summary><div class="bubble" style="margin-top:8px"><${Markdown} text=${d.profile} /></div></details>`}
      ${d.notes && html`<details><summary class="section-title" style="cursor:pointer"><${Icon} name="file" /><h3>Ghi chú soát lời</h3></summary><div class="bubble" style="margin-top:8px"><${Markdown} text=${d.notes} /></div></details>`}
      <${AgentAction} slug=${slug} action="suno.agent" title="Điền form suno.com/create" hint="Agent dùng CUA điều khiển Chrome; không bấm Create, bạn tự xem và bấm" />
      <div class="row wrap"><${Btn} icon="external" onClick=${() => window.open("https://suno.com/create", "_blank", "noopener")}>Mở Suno</${Btn}>
        <span class="muted small">Tạo xong, tải bản <b>M4A</b> (có sẵn timing lời) rồi sang bước Nhạc.</span></div>
    `}
  </div>`;
}

function AudioStep({ d, slug, reload, stamp }) {
  const [recent, setRecent] = useState([]);
  const [over, setOver] = useState(false);
  const [busy, setBusy] = useState(false);
  const [path, setPath] = useState("");
  useEffect(() => { api("/api/downloads/audio").then(setRecent).catch(() => {}); }, []);
  const importPath = async (p) => {
    setBusy(true);
    try { await api(`/api/projects/${encodeURIComponent(slug)}/audio`, { method: "POST", body: { path: p } }); toast("Đã thêm nhạc"); bumpRuns(); await reload(); }
    catch (e) { toastError(e); } finally { setBusy(false); }
  };
  const upload = async (file) => {
    const fd = new FormData();
    fd.append("file", file);
    setBusy(true);
    try { await api(`/api/projects/${encodeURIComponent(slug)}/audio`, { method: "POST", body: fd }); toast("Đã tải lên"); await reload(); }
    catch (e) { toastError(e); } finally { setBusy(false); }
  };
  return html`<div class="stack">
    ${d.audio && html`<div><div class="row" style="margin-bottom:8px"><${Icon} name="music" /><b>${d.audio}</b></div><audio controls preload="metadata" src=${media(slug, d.audio, stamp)}></audio></div>`}
    ${d.audio && !stepDone(d, "render") && html`<${VideoCTA} d=${d} slug=${slug} />`}
    ${d.audio && html`<div class="muted small">Muốn đổi bản nhạc khác? Kéo file mới vào bên dưới.</div>`}
    <div class=${"dropzone" + (over ? " over" : "")} onDragOver=${(e) => { e.preventDefault(); setOver(true); }} onDragLeave=${() => setOver(false)}
      onDrop=${(e) => { e.preventDefault(); setOver(false); const f = e.dataTransfer.files[0]; f && upload(f); }}>
      <${Icon} name="upload" size=${24} /><div style="margin:8px 0 4px;font-weight:600">Kéo file M4A từ Suno vào đây</div>
      <div class="muted small">hoặc <label style="color:var(--accent);cursor:pointer">chọn file<input type="file" accept="audio/*,.m4a" hidden onChange=${(e) => e.target.files[0] && upload(e.target.files[0])} /></label></div>
      ${busy && html`<div style="margin-top:8px"><${Icon} name="loader" class="spin" /></div>`}
    </div>
    ${recent.length > 0 && html`<div><h3 style="margin-bottom:8px">Mới tải trong Downloads</h3><div class="file-list">
      ${recent.map((f) => html`<div class="file-item" onClick=${() => importPath(f.path)}><${Icon} name="music" /><div class="grow"><div style="font-weight:600">${f.name}</div>
        <div class="muted small">${bytes(f.size)} · ${ago(f.modified)}</div></div><span class="pill accent">Dùng file này</span></div>`)}</div></div>`}
    <div class="row"><input class="input" placeholder="hoặc dán đường dẫn file, ví dụ ~/Downloads/Bai hat.m4a" value=${path} onInput=${(e) => setPath(e.target.value)} />
      <${Btn} disabled=${!path.trim()} busy=${busy} onClick=${() => importPath(path)}>Thêm</${Btn}></div>
  </div>`;
}

function AlignStep({ d, slug }) {
  return html`<div class="stack">
    <${ActionCard} title="Canh lời từng chữ" hint="Lấy phụ đề Suno trong M4A, tách giọng bằng Demucs, Whisper canh từng chữ rồi khớp theo giọng thật" onRun=${() => runAction(slug, "align")} label="Canh lời" />
    ${!d.audio && html`<div class="warn-box">Cần thêm file nhạc ở bước 3 trước.</div>`}
    ${d.timings.length > 0 && html`<div class="scroll-x" style="max-height:460px"><table class="table"><thead><tr><th>#</th><th>Bắt đầu</th><th>Kết thúc</th><th>Lệch Suno</th><th>Lời</th></tr></thead><tbody>
      ${d.timings.map((l, i) => html`<tr><td class="num">${i + 1}</td><td class="num">${secs(l.start)}</td><td class="num">${secs(l.end)}</td>
        <td class="num">${Math.abs(l.drift) > 1 ? html`<span class="pill warn">${l.drift.toFixed(2)}s</span>` : l.drift.toFixed(2) + "s"}${l.source === "even" ? html` <span class="pill warn">chia đều</span>` : ""}</td><td>${l.text}</td></tr>`)}
    </tbody></table></div>`}
  </div>`;
}

function TrimStep({ d, slug, stamp }) {
  const [at, setAt] = useState(11.5);
  const cfg = safeJSON(d.config);
  const first = d.timings[0]?.start;
  return html`<div class="stack">
    <dl class="kv"><dt>Giọng vào ở</dt><dd><b>${first != null ? secs(first) : "—"}</b> ${first > 15 ? html`<span class="pill warn">dài hơn bài mẫu (~11s)</span>` : first != null ? html`<span class="pill ok">ổn</span>` : ""}</dd>
      ${cfg.intro_cut && html`<dt>Đã cắt</dt><dd class="mono">${secs(cfg.intro_cut[0])} → ${secs(cfg.intro_cut[1])}</dd>`}</dl>
    ${!cfg.intro_cut && html`<${ActionCard} title="Cắt các câu nhạc dạo" hint="Bỏ trọn các câu 8 phách, chọn chỗ nối hợp âm giống nhau nhất, nối mềm 0,25s" onRun=${() => runAction(slug, "trim", { vocalAt: Number(at) })} label="Cắt intro">
      <label class="row small muted">Giọng vào ở <input class="input" type="number" step="0.5" min="5" max="30" style="width:84px" value=${at} onInput=${(e) => setAt(e.target.value)} /> giây</label>
    </${ActionCard}>`}
    ${cfg.intro_cut && html`<div class="muted small">Để cắt lại, thêm lại file M4A gốc ở bước 3 (việc đó xoá mốc cắt cũ) rồi canh lời lại.</div>`}
    ${d.audio && html`<audio controls preload="metadata" src=${media(slug, d.audio, stamp)}></audio>`}
  </div>`;
}

function ScenesStep({ d, slug, reload, stamp }) {
  const [box, setBox] = useState(null);
  const [editing, setEditing] = useState(false);
  const cfg = safeJSON(d.config);
  const scenes = cfg.scenes || [];
  const imgs = new Set(d.images);
  return html`<div class="stack">
    <${AgentAction} slug=${slug} action="scenes.agent" title="Lên danh sách cảnh theo lời" hint="Ghi scenes vào project.json: mỗi cảnh đổi ở đầu một câu, prompt tiếng Anh nêu đúng địa danh" />
    <${ActionCard} title="Tạo ảnh còn thiếu" hint=${`Model ${cfg.image_model || "gpt-image-2.5"} · 4 ảnh song song · ảnh đã có được giữ, xoá file để tạo lại`} onRun=${() => runAction(slug, "images")} label="Tạo ảnh" icon="image" />
    <div class="row"><h3>${scenes.length} cảnh</h3><span class="spacer"></span><${Btn} size="sm" icon="code" onClick=${() => setEditing(true)}>Sửa project.json</${Btn}></div>
    ${scenes.length > 0 && html`<div class="scroll-x" style="max-height:380px"><table class="table"><thead><tr><th>Bắt đầu</th><th>Cảnh</th><th>Prompt</th><th></th></tr></thead><tbody>
      ${scenes.map((s) => { const img = `img/${s.id}.png`; return html`<tr><td class="num">${secs(s.start)}</td><td class="mono small">${s.id}</td>
        <td class="small">${s.reuse ? html`<span class="muted">dùng lại ảnh tựa đề</span>` : (s.prompt || "").slice(0, 160)}</td>
        <td>${imgs.has(img) ? html`<img src=${media(slug, img, stamp)} style="width:72px;border-radius:6px;cursor:zoom-in" onClick=${() => setBox(img)} />` : s.reuse ? "" : html`<span class="pill">chưa có</span>`}</td></tr>`; })}
    </tbody></table></div>`}
    ${d.images.length > 0 && html`<div class="gallery">${d.images.map((img) => html`<figure onClick=${() => setBox(img)}><img loading="lazy" src=${media(slug, img, stamp)} /><figcaption>${img.replace("img/", "")}</figcaption></figure>`)}</div>`}
    ${box && html`<${Lightbox} src=${media(slug, box, stamp)} caption=${box} onClose=${() => setBox(null)} />`}
    ${editing && html`<${JsonEditor} d=${d} slug=${slug} reload=${reload} onClose=${() => setEditing(false)} />`}
  </div>`;
}

function JsonEditor({ d, slug, reload, onClose }) {
  const [val, setVal] = useState(JSON.stringify(safeJSON(d.config), null, 1));
  const [busy, save] = useSave(slug, reload);
  let bad = null;
  try { JSON.parse(val); } catch (e) { bad = e.message; }
  return html`<${Modal} title="project.json" onClose=${onClose} footer=${html`${bad && html`<span class="small" style="color:var(--bad)">${bad}</span>`}<span class="spacer"></span>
      <${Btn} kind="ghost" onClick=${onClose}>Đóng</${Btn}><${Btn} kind="primary" icon="save" busy=${busy} disabled=${!!bad} onClick=${async () => { await save("project.json", val, d.configSha); onClose(); }}>Lưu</${Btn}>`}>
    <textarea class="textarea mono" style="min-height:60vh" value=${val} onInput=${(e) => setVal(e.target.value)}></textarea>
  </${Modal}>`;
}

function StillsStep({ d, slug, stamp }) {
  const [box, setBox] = useState(null);
  const [times, setTimes] = useState("");
  const parsed = times.split(/[\s,]+/).map(Number).filter((n) => isFinite(n) && n >= 0 && times.trim());
  return html`<div class="stack">
    <${ActionCard} title="Render khung hình xem trước" hint="Mặc định: tựa đề, đếm ngược micro, giữa câu dài nhất, đoạn kết. Nhanh, vài giây mỗi khung"
      onRun=${() => runAction(slug, "stills", parsed.length ? { times: parsed } : {})} label="Render" icon="eye">
      <input class="input" style="width:200px" placeholder="giây tuỳ chọn: 8, 12.5, 90" value=${times} onInput=${(e) => setTimes(e.target.value)} />
    </${ActionCard}>
    ${d.stills.length > 0 ? html`<div class="gallery">${d.stills.map((s) => html`<figure onClick=${() => setBox(s)}><img loading="lazy" src=${media(slug, s, stamp)} />
      <figcaption>${secs(parseFloat(s.replace(/.*still-/, "")))}</figcaption></figure>`)}</div>` : html`<${Empty} icon="eye" title="Chưa có khung hình">Render vài khung để kiểm tra chữ, micro và cảnh trước khi xuất cả video.</${Empty}>`}
    ${box && html`<${Lightbox} src=${media(slug, box, stamp)} caption=${box} onClose=${() => setBox(null)} />`}
  </div>`;
}

const SHEET_KEYS = [["orig", "Như giọng hát"], ["C", "Đô (C)"], ["D", "Rê (D)"], ["E", "Mi (E)"], ["F", "Fa (F)"], ["G", "Sol (G)"], ["A", "La (A)"], ["Bb", "Si giáng (Bb)"], ["Eb", "Mi giáng (Eb)"]];

// SheetStep writes a lead sheet (melody + lyrics) from the sung vocal, like a printed song sheet.
function SheetStep({ d, slug, stamp }) {
  const sh = d.sheet;
  const [meter, setMeter] = useState(sh?.meter || "4/4");
  const [key, setKey] = useState(sh?.targetKey || "orig");
  const [box, setBox] = useState(null);
  const open = (path, extra = {}) => api(`/api/projects/${encodeURIComponent(slug)}/open`, { method: "POST", body: { path, ...extra } }).catch(toastError);
  return html`<div class="stack">
    ${!d.timings.length && html`<div class="warn-box">Cần canh lời trước (<a href=${`#/p/${slug}/align`} style="color:inherit;text-decoration:underline">bước 4</a>): nhịp của bản nhạc lấy từ thời điểm hát từng chữ.</div>`}
    <${ActionCard} title="Chép bản nhạc" hint="Lấy cao độ từ giọng hát đã tách và tiết tấu từ thời điểm từng chữ, viết thành bản nhạc có lời như bản in. Bản tự động, nên mở bằng MuseScore để sửa vài chỗ trước khi in"
      onRun=${() => runAction(slug, "sheet", { meter, key })} label=${sh ? "Chép lại" : "Chép bản nhạc"} icon="sheet">
      <div class="row wrap" style="gap:8px">
        <${Seg} value=${meter} onChange=${setMeter} options=${["2/4", "3/4", "4/4"].map((m) => ({ value: m, label: m }))} />
        <select class="select" value=${key} onChange=${(e) => setKey(e.target.value)} aria-label="Giọng">
          ${SHEET_KEYS.map(([v, l]) => html`<option value=${v}>${v === "orig" ? l : "Dịch sang " + l}</option>`)}
        </select>
      </div>
    </${ActionCard}>
    ${sh ? html`
      <div class="row wrap" style="gap:8px">
        <span class="pill">Giọng ${sh.key}</span><span class="pill">Nhịp ${sh.meter}</span><span class="pill">♩ = ${Math.round(sh.bpm)}</span>
        <span class="pill" title="Phần giọng hát mà nốt viết ra khớp trong vòng nửa cung">Khớp cao độ ${Math.round(sh.pitchMatch * 100)}%</span>
      </div>
      <div class="row wrap">
        ${sh.pdf && html`<${Btn} size="sm" kind="primary" icon="sheet" onClick=${() => open(sh.pdf)}>Mở PDF</${Btn}>`}
        <${Btn} size="sm" icon="edit" onClick=${() => open(sh.xml, { app: "musescore" })}>Sửa trong MuseScore</${Btn}>
        <${Btn} size="sm" icon="folder" onClick=${() => open(sh.pdf || sh.xml, { reveal: true })}>Hiện trong Finder</${Btn}>
      </div>
      ${sh.pages.length ? html`<div class="sheet-pages">${sh.pages.map((p, i) => html`<figure onClick=${() => setBox(p)}><img loading="lazy" src=${media(slug, p, stamp)} alt=${`Trang ${i + 1}`} /><figcaption>Trang ${i + 1}</figcaption></figure>`)}</div>`
        : html`<div class="muted small">Chưa có ảnh xem trước: cài MuseScore (brew install --cask musescore) rồi chép lại để có PDF.</div>`}
      <div class="muted small">Sửa xong trong MuseScore, dùng File → Export → PDF để in. Chép lại ở đây sẽ ghi đè bản đã sửa.</div>`
      : html`<${Empty} icon="sheet" title="Chưa có bản nhạc">Bản nhạc được chép tự động khi chạy chuỗi video, hoặc bấm Chép bản nhạc.</${Empty}>`}
    ${box && html`<${Lightbox} src=${media(slug, box, stamp)} caption=${box} onClose=${() => setBox(null)} />`}
  </div>`;
}

// Deletes move files to the macOS Trash, so a slip can be undone with Finder's "Put Back".
async function trashProject(d, slug) {
  const name = [d.title, d.subtitle].filter(Boolean).join(" – ");
  if (!confirm(`Chuyển cả bài "${name}" vào Thùng rác?\n\nThư mục ${d.dir} (lời, nhạc, ảnh, video) sẽ nằm trong Thùng rác, lấy lại được bằng Finder cho tới khi bạn dọn Thùng rác.`)) return;
  try {
    await api(`/api/projects/${encodeURIComponent(slug)}`, { method: "DELETE" });
    toast("Đã chuyển bài vào Thùng rác");
    go("/");
  } catch (e) { toastError(e); }
}

function RenderStep({ d, slug, stamp, reload }) {
  const video = d.videos[d.videos.length - 1];
  const open = (reveal) => api(`/api/projects/${encodeURIComponent(slug)}/open`, { method: "POST", body: { path: video, reveal } }).catch(toastError);
  const trash = async () => {
    if (!confirm(`Chuyển video "${video}" vào Thùng rác?\n\nBạn có thể xuất lại bất cứ lúc nào; bản đã chép sang Downloads (nếu có) không bị đụng tới.`)) return;
    try { await api(`/api/projects/${encodeURIComponent(slug)}/video?path=${encodeURIComponent(video)}`, { method: "DELETE" }); toast("Đã chuyển video vào Thùng rác"); reload(); }
    catch (e) { toastError(e); }
  };
  return html`<div class="stack">
    ${video ? html`<video controls preload="metadata" src=${media(slug, video, stamp)}></video>
      <div class="row wrap"><b class="grow">${video}</b><${Btn} size="sm" icon="play" onClick=${() => open(false)}>Mở bằng QuickTime</${Btn}><${Btn} size="sm" icon="folder" onClick=${() => open(true)}>Hiện trong Finder</${Btn}><${Btn} size="sm" kind="danger" icon="trash" onClick=${trash}>Xoá video</${Btn}></div>`
      : html`<${Empty} icon="film" title="Chưa có video">Xuất video sau khi đã xem trước khung hình.</${Empty}>`}
    <${ChainStatus} slug=${slug} steps=${["align", "trim", "scenes", "stills", "render", "sheet"]} title="Chuỗi video" />
    ${!d.audio && html`<div class="warn-box">Chưa có nhạc. Thêm file M4A từ Suno ở <a href=${`#/p/${slug}/audio`} style="color:inherit;text-decoration:underline">bước 3</a> trước.</div>`}
    <${ActionCard} agent title="Làm video tự động" hint="Canh lời → cắt intro nếu cần → agent lên cảnh → tạo ảnh còn thiếu → khung xem trước → xuất video. Canh lời, cắt intro và lên cảnh được bỏ qua khi đã làm và vẫn còn đúng" busy=${d.steps.some((s) => s.status === "running")} onRun=${() => runPipeline(slug, "video")} label="Chạy cả chuỗi" icon="zap" />
    <${ActionCard} title="Chỉ xuất video" hint="1920×1080, 30 fps, H.264 + AAC. Khoảng 8–9 phút cho bài 4 phút" onRun=${() => runAction(slug, "render")} label="Xuất video" icon="film" />
  </div>`;
}

function RecentRuns({ slug, step }) {
  const [runs, setRuns] = useState([]);
  const runsVersion = useStore((s) => s.runsVersion);
  useEffect(() => { api(`/api/runs?project=${encodeURIComponent(slug)}&limit=40`).then(setRuns).catch(() => {}); }, [slug, runsVersion]);
  const mine = runs.filter((r) => r.step === step).slice(0, 6);
  if (!mine.length) return null;
  return html`<div class="card"><div class="card-head"><h3>Lượt chạy của bước này</h3></div>
    <table class="table runs-table"><tbody>${mine.map((r) => html`<tr onClick=${() => openRun(r.id)}>
      <td style="width:30px"><${AgentIcon} agent=${r.kind === "agent" ? r.agent : "script"} /></td><td>${r.title}</td>
      <td><${StatusPill} status=${r.status} /></td><td class="num muted">${runDuration(r)}</td><td class="muted small">${ago(r.created)}</td></tr>`)}</tbody></table></div>`;
}

function FreeAgentButton({ slug }) {
  const [open, setOpen] = useState(false);
  const ag = useAgentChoice();
  const [prompt, setPrompt] = useState("");
  const send = async () => {
    try {
      const meta = await api("/api/runs", { method: "POST", body: { target: "project", project: slug, agent: ag.agent, model: ag.model, prompt } });
      openRun(meta.id); bumpRuns(); setOpen(false); setPrompt("");
    } catch (e) { toastError(e); }
  };
  return html`<${Btn} size="sm" icon="bot" onClick=${() => setOpen(true)}>Giao việc cho agent</${Btn}>
    ${open && html`<${Modal} title="Giao việc tự do cho agent" onClose=${() => setOpen(false)} footer=${html`<${Btn} kind="ghost" onClick=${() => setOpen(false)}>Huỷ</${Btn}>
      <${Btn} kind="primary" icon="send" disabled=${!prompt.trim()} onClick=${send}>Gửi</${Btn}>`}>
      <div class="stack"><${AgentPicker} value=${ag.agent} onChange=${ag.setAgent} model=${ag.model} onModel=${ag.setModel} />
        <${Field} label="Việc cần làm" hint="Agent làm trong thư mục bài này và đọc skill suno-que-huong trước."><textarea class="textarea" rows="7" autofocus placeholder="Ví dụ: đổi cảnh 07 thành ảnh chùa Bái Đính lúc bình minh rồi tạo lại ảnh đó" value=${prompt} onInput=${(e) => setPrompt(e.target.value)}></textarea></${Field}>
        <${PresetChips} value=${prompt} onChange=${setPrompt} /></div>
    </${Modal}>`}`;
}

function safeJSON(raw) {
  if (raw && typeof raw === "object") return raw;
  try { return JSON.parse(raw || "{}"); } catch { return {}; }
}
