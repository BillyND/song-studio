// Step 1: lyrics, genre, references and brief, then build the Suno prompt.
import { api, go, html, t, toastError, useEffect, useState } from "../core.js";
import { Icon } from "../icons.js";
import { AgentPicker, Btn, GenrePicker, PresetChips, Switch, useAgentChoice } from "../ui.js";
import { runPipeline, safeJSON, useSave } from "./actions.js";
import { RefsPanel } from "./refs.js";

// looksLucBat: most lines have 6 or 8 syllables, so the lục bát checker is worth offering.
function looksLucBat(text) {
  const lines = text.split("\n").map((l) => l.replace(/[,.!?;:"“”]/g, " ").trim()).filter(Boolean);
  if (lines.length < 4) return false;
  const fit = lines.filter((l) => [6, 7, 8, 9].includes(l.split(/\s+/).length)).length;
  return fit / lines.length >= 0.6;
}

export function LyricsStep({ d, slug, reload, stamp }) {
  const [text, setText] = useState(d.lyrics);
  const [brief, setBrief] = useState(d.brief);
  const [check, setCheck] = useState(null);
  const [fixAll, setFixAll] = useState(false);
  const ag = useAgentChoice();
  const [busy, save] = useSave(slug, reload);
  const cfg = safeJSON(d.config);
  const setMeta = (v) => api(`/api/projects/${encodeURIComponent(slug)}/meta`, { method: "PATCH", body: v }).then(reload).catch(toastError);
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
    <div class="section-title"><${Icon} name="type" /><h3>${t("Lời bài hát")}</h3><span class="spacer"></span>
      ${(cfg.language || "vi") === "vi" && looksLucBat(text) && html`<${Btn} size="sm" icon="check" onClick=${runCheck}>${t("Kiểm tra lục bát")}</${Btn}>`}</div>
    <${GenrePicker} genre=${cfg.genre} language=${cfg.language} onChange=${setMeta} />
    <textarea class="textarea" rows="12" value=${text} onInput=${(e) => setText(e.target.value)} onKeyDown=${(e) => (e.metaKey || e.ctrlKey) && e.key === "s" && (e.preventDefault(), saveAll())}></textarea>
    ${check && html`<div class="card flat"><div class="card-head"><h3>${t("Lục bát")}</h3><span class=${"pill " + (issues ? "warn" : "ok")}>${issues ? t("{n} dòng cần xem", { n: issues }) : t("Ổn")}</span></div>
      <div class="scroll-x" style="max-height:320px"><table class="table"><thead><tr><th>#</th><th>${t("Lời")}</th><th>${t("Chữ")}</th><th>${t("Ghi chú")}</th></tr></thead><tbody>
      ${check.map((r) => html`<tr><td class="num">${r.line}</td><td>${r.text}</td><td class="num">${r.syllables} · ${r.kind}</td>
        <td class="small">${r.notes.length ? r.notes.map((n) => html`<div class=${n.startsWith("có tiếng đệm") ? "muted" : ""}>${n}</div>`) : html`<span class="pill ok">ok</span>`}</td></tr>`)}
      </tbody></table></div></div>`}

    <div class="section-title"><${Icon} name="music" /><h3>${t("Bài mẫu")}</h3><span class="muted small">${t("không bắt buộc · chỉ học phong cách, không gửi lên Suno")}</span></div>
    <${RefsPanel} d=${d} slug=${slug} reload=${reload} stamp=${stamp} />

    <div class="section-title"><${Icon} name="wand" /><h3>${t("Yêu cầu mở rộng")}</h3><span class="muted small">${t("không bắt buộc")}</span></div>
    <textarea class="textarea" rows="4" placeholder=${t("Ví dụ: phong cách dân ca mượt mà nhưng không ề à, không dùng bộ gõ…")} value=${brief}
      onInput=${(e) => setBrief(e.target.value)} onKeyDown=${(e) => (e.metaKey || e.ctrlKey) && e.key === "s" && (e.preventDefault(), saveAll())}></textarea>
    <${PresetChips} genre=${cfg.genre} value=${brief} onChange=${setBrief} />

    <div class="cta">
      <div class="cta-fields">
        <label class="field"><span>Agent</span><${AgentPicker} value=${ag.agent} onChange=${ag.setAgent} model=${ag.model} onModel=${ag.setModel} /></label>
        <${Switch} on=${fixAll} onChange=${setFixAll}>${t("Sửa hết đề xuất cấu trúc lời")}</${Switch}>
      </div>
      <div class="cta-go">
        <div class="muted small">${d.refs.some((r) => !r.analysis) ? t("Phân tích bài mẫu ‖ soát lời & chia đoạn → phối khí & Styles") : t("Soát lời & chia đoạn → phối khí & Styles")}</div>
        <div class="row wrap">
          ${dirty && html`<${Btn} icon="save" busy=${busy} onClick=${saveAll}>${t("Lưu")}</${Btn}>`}
          <${Btn} kind="primary" icon="sparkles" busy=${starting} disabled=${!text.trim() || chainRunning} onClick=${generate}>${chainRunning ? t("Đang tạo prompt…") : t("Tạo prompt Suno")}</${Btn}>
        </div>
      </div>
    </div>
  </div>`;
}
