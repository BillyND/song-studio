// Building blocks shared by the step panels: action rows, agent actions, chains, copy blocks.
import { api, copyText, go, html, openRun, render, t, useEffect, useState, useStore } from "../core.js";
import { Icon } from "../icons.js";
import { AgentIcon, AgentPicker, Btn, PresetChips, StatusPill, useAgentChoice } from "../ui.js";
import { goStep, runAction, runPipeline, stepDone } from "./actions.js";

// ActionCard is one runnable action of a step. Its button stays secondary so that the one
// primary button on the screen (the header's next step) stands out.
export function ActionCard({ title, hint, children, onRun, label, busy, icon = "play", agent }) {
  return html`<div class="action-bar">
    <div class="grow" style="min-width:220px"><div class="row" style="gap:8px">${agent ? html`<${Icon} name="bot" size=${15} />` : html`<${Icon} name="terminal" size=${15} />`}<b>${title}</b></div>
      ${hint && html`<div class="muted small">${hint}</div>`}</div>
    ${children}
    <${Btn} icon=${icon} busy=${busy} onClick=${onRun}>${label || t("Chạy")}</${Btn}>
  </div>`;
}

export function AgentAction({ slug, action, title, hint, extra, body = {} }) {
  const ag = useAgentChoice();
  const [note, setNote] = useState("");
  return html`<div class="stack" style="gap:10px">
    <${ActionCard} agent title=${title} hint=${hint} label=${t("Giao cho agent")} icon="send"
      onRun=${() => runAction(slug, action, { agent: ag.agent, model: ag.model, note, ...body })}>
      <${AgentPicker} value=${ag.agent} onChange=${ag.setAgent} model=${ag.model} onModel=${ag.setModel} />
    </${ActionCard}>
    ${extra}
    <textarea class="textarea" rows="2" style="min-height:44px" placeholder=${t("Dặn thêm agent (không bắt buộc)…")} value=${note} onInput=${(e) => setNote(e.target.value)}></textarea>
    <${PresetChips} value=${note} onChange=${setNote} />
  </div>`;
}

// ChainStatus shows the newest chain of runs for these steps, e.g. analysis ‖ lyrics → arrangement.
export function ChainStatus({ slug, steps, title }) {
  const [runs, setRuns] = useState([]);
  const runsVersion = useStore((s) => s.runsVersion);
  const load = () => api(`/api/runs?project=${encodeURIComponent(slug)}&limit=60`).then(setRuns).catch(() => {});
  useEffect(() => { load(); }, [slug, runsVersion]);
  const group = runs.find((r) => r.group && steps.includes(r.step))?.group;
  const chain = group ? runs.filter((r) => r.group === group).reverse() : [];
  const live = chain.some((r) => r.status === "running" || r.status === "queued");
  useEffect(() => { if (!live) return; const timer = setInterval(load, 1500); return () => clearInterval(timer); }, [live, group]);
  if (!chain.length) return null;
  return html`<div class="chain">
    <span class="muted small" style="font-weight:650">${title}</span>
    ${chain.map((r, i) => html`${i > 0 && html`<${Icon} name="chevron" size=${12} class="muted" />`}
      <button type="button" class="chain-step" onClick=${() => openRun(r.id)} title=${r.title}>
        <${AgentIcon} agent=${r.kind === "agent" ? r.agent : "script"} /><span>${r.title}</span><${StatusPill} status=${r.status} /></button>`)}
  </div>`;
}

export function CopyBlock({ title, text, extra, onSave, saving }) {
  const [edit, setEdit] = useState(false);
  const [val, setVal] = useState(text);
  useEffect(() => setVal(text), [text]);
  return html`<div class="stack" style="gap:8px">
    <div class="row wrap"><h3>${title}</h3>${extra}<span class="spacer"></span>
      ${onSave && html`<${Btn} size="sm" kind="ghost" icon=${edit ? "x" : "code"} onClick=${() => setEdit(!edit)}>${edit ? t("Huỷ") : t("Sửa")}</${Btn}>`}
      ${edit ? html`<${Btn} size="sm" icon="save" busy=${saving} onClick=${async () => { await onSave(val); setEdit(false); }}>${t("Lưu")}</${Btn}>`
        : html`<${Btn} size="sm" icon="copy" onClick=${() => copyText(text)}>${t("Chép")}</${Btn}>`}</div>
    ${edit ? html`<textarea class="textarea mono" rows="10" value=${val} onInput=${(e) => setVal(e.target.value)}></textarea>` : html`<pre class="block">${text}</pre>`}
  </div>`;
}

export function Stat({ k, v, s }) {
  return v ? html`<div class="stat"><div class="k">${k}</div><div class="v">${v}</div>${s && html`<div class="s">${s}</div>`}</div>` : null;
}

// VideoCTA explains and starts the whole video chain in one click.
export function VideoCTA({ d, slug }) {
  const left = ["align", "scenes", "stills", "render", "sheet"].filter((id) => !stepDone(d, id)).length;
  return html`<div class="cta">
    <div><div class="row" style="gap:8px"><${Icon} name="film" /><b>${t("Làm video karaoke từ bài này")}</b></div>
      <div class="muted small" style="margin-top:4px">${t("Studio tự canh lời theo giọng hát → agent lên danh sách cảnh → tạo ảnh AI → render khung xem trước → xuất video 1080p, kèm bản nhạc có lời. Còn {n} bước, khoảng 15–25 phút; bạn có thể đóng trang, việc vẫn chạy.", { n: left })}</div></div>
    <div class="cta-go"><${Btn} kind="primary" icon="zap" busy=${d.steps.some((s) => s.status === "running")} onClick=${async () => { if (await runPipeline(slug, "video")) goStep(slug, "render"); }}>${t("Làm video tự động")}</${Btn}></div>
  </div>`;
}
