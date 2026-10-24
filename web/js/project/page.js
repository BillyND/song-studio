// One song's workspace: header, step list, the open step's panel and its recent runs.
import { ago, api, go, html, media, openChat, openRun, render, runDuration, t, toastError, useEffect, useState, useStore } from "../core.js";
import { Icon } from "../icons.js";
import { AgentIcon, Btn, Empty, StatusPill } from "../ui.js";
import { goStep, runPipeline, stepDone, trashProject } from "./actions.js";
import { AudioStep } from "./audio.js";
import { LyricsStep } from "./lyrics.js";
import { ScenesStep } from "./scenes.js";
import { SheetStep } from "./sheet.js";
import { SunoStep } from "./suno.js";
import { AlignStep, RenderStep, StillsStep } from "./video.js";

const STEP_ICON = { lyrics: "type", suno: "sparkles", audio: "music", align: "mic", scenes: "image", stills: "eye", render: "film", sheet: "sheet" };

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
    const timer = setInterval(load, 2500);
    return () => clearInterval(timer);
  }, [running, slug]);

  if (err) return html`<div class="page"><${Empty} icon="alert" title=${t("Không mở được bài")}>${err.message}</${Empty}></div>`;
  if (!d) return html`<div class="page"><div class="empty"><${Icon} name="loader" class="spin" size=${22} /></div></div>`;
  // a step from an old link (for example the removed "trim" step) falls back to the next useful one
  const known = step && d.steps.some((s) => s.id === step);
  const current = known ? step : (d.steps.find((s) => s.status === "todo" || s.status === "stale") || d.steps[d.steps.length - 1]).id;
  const ctx = { d, slug, reload: load, stamp };
  const S = { lyrics: LyricsStep, suno: SunoStep, audio: AudioStep, align: AlignStep, scenes: ScenesStep, stills: StillsStep, render: RenderStep, sheet: SheetStep }[current] || LyricsStep;
  const stepMeta = d.steps.find((s) => s.id === current);
  return html`<div class="page">
    <div class="hero">
      <div class="hero-bg" style=${d.cover ? { backgroundImage: `url("${media(slug, d.cover, stamp)}")` } : {}}></div>
      <div class="hero-in">
        <div class="grow">
          <a class="small muted" href="#/">${t("← Bài hát")}</a>
          <h1>${d.title} <span class="subtitle">${d.subtitle}</span></h1>
          <div class="muted small">${t("{done}/{total} bước xong · cập nhật {ago}", { done: d.done, total: d.total, ago: ago(d.updated) })}<span class="hero-path"> · <span class="mono">${d.dir}</span></span></div>
        </div>
        <div class="hero-actions">
          <${Btn} size="sm" icon="folder" onClick=${() => api(`/api/projects/${slug}/open`, { method: "POST", body: { path: "" } }).catch(toastError)}>${t("Mở thư mục")}</${Btn}>
          <${FreeAgentButton} />
          <${Btn} size="sm" icon="trash" title=${t("Xoá bài")} onClick=${() => trashProject(d, slug)} />
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
            ${stepMeta.runId && html`<${Btn} size="sm" icon="terminal" onClick=${() => openRun(stepMeta.runId)}>${t("Xem đang chạy")}</${Btn}>`}</div>
          <div class="muted small" style="margin-bottom:16px">${stepMeta.detail}</div>
          <${S} ...${ctx} />
          <${NextBar} d=${d} slug=${slug} current=${current} />
        </div>
        <${RecentRuns} slug=${slug} step=${current} />
      </div>
    </div>
  </div>`;
}

// PrimaryAction is the single "do the next thing" button in the song header.
function PrimaryAction({ d, slug }) {
  const running = d.steps.some((s) => s.status === "running");
  if (running) return html`<${Btn} size="sm" kind="primary" icon="loader" onClick=${() => openRun(d.steps.find((s) => s.status === "running").runId)}>${t("Đang chạy…")}</${Btn}>`;
  if (!d.audio && !stepDone(d, "suno")) return html`<${Btn} size="sm" kind="primary" icon="sparkles" onClick=${() => goStep(slug, "lyrics")}>${t("Tạo prompt Suno")}</${Btn}>`;
  if (!d.audio) return html`<${Btn} size="sm" kind="primary" icon="music" onClick=${() => goStep(slug, "audio")}>${t("Thêm nhạc từ Suno")}</${Btn}>`;
  if (!stepDone(d, "render")) return html`<${Btn} size="sm" kind="primary" icon="film" onClick=${async () => { if (await runPipeline(slug, "video")) goStep(slug, "render"); }}>${t("Làm video")}</${Btn}>`;
  return html`<${Btn} size="sm" kind="primary" icon="play" onClick=${() => goStep(slug, "render")}>${t("Xem video")}</${Btn}>`;
}

// NextBar sits under every step and says what comes next.
function NextBar({ d, slug, current }) {
  const i = d.steps.findIndex((s) => s.id === current);
  const next = d.steps[i + 1];
  if (!next) return null;
  const here = d.steps[i];
  const ready = here.status === "done" || here.status === "skip";
  return html`<div class="next-bar">
    <span class="muted small">${ready ? t("Bước này đã xong.") : t("Làm xong bước này rồi sang:")}</span>
    <span class="spacer"></span>
    <${Btn} kind=${ready ? "primary" : ""} icon="chevron" onClick=${() => goStep(slug, next.id)}>${t("Tiếp: {step}", { step: next.title })}</${Btn}>
  </div>`;
}

function RecentRuns({ slug, step }) {
  const [runs, setRuns] = useState([]);
  const runsVersion = useStore((s) => s.runsVersion);
  useEffect(() => { api(`/api/runs?project=${encodeURIComponent(slug)}&limit=40`).then(setRuns).catch(() => {}); }, [slug, runsVersion]);
  const mine = runs.filter((r) => r.step === step).slice(0, 6);
  if (!mine.length) return null;
  return html`<div class="card"><div class="card-head"><h3>${t("Lượt chạy của bước này")}</h3></div>
    <table class="table runs-table"><tbody>${mine.map((r) => html`<tr onClick=${() => openRun(r.id)}>
      <td style="width:30px"><${AgentIcon} agent=${r.kind === "agent" ? r.agent : "script"} /></td><td>${r.title}</td>
      <td><${StatusPill} status=${r.status} /></td><td class="num muted">${runDuration(r)}</td><td class="muted small">${ago(r.created)}</td></tr>`)}</tbody></table></div>`;
}

// Starts a new agent chat about this song in the side panel.
function FreeAgentButton() {
  return html`<${Btn} size="sm" icon="bot" title=${t("Chat với agent về bài này (⌘J)")} onClick=${() => openChat(null)}>${t("Chat với agent")}</${Btn}>`;
}
