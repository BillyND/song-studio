// Skill workspace: browse and edit the skill files, or ask an agent to change them.
import { html, useState, useEffect, useRef, api, toast, toastError, openRun, bumpRuns, ago, bytes, useStore, runDuration } from "./core.js";
import { Icon } from "./icons.js";
import { Btn, AgentPicker, useAgentChoice, StatusPill, AgentIcon, Modal, PresetChips } from "./ui.js";

export function SkillPage() {
  const [tree, setTree] = useState(null);
  const [path, setPath] = useState("SKILL.md");
  const [file, setFile] = useState(null);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [conflict, setConflict] = useState(false);
  const runsVersion = useStore((s) => s.runsVersion);
  const dirty = file && text !== file.content;

  const loadTree = () => api("/api/skill/tree").then(setTree).catch(toastError);
  const loadFile = async (p, force) => {
    if (!force && dirty && !confirm("Bỏ thay đổi chưa lưu?")) return;
    try { const f = await api(`/api/skill/file?path=${encodeURIComponent(p)}`); setFile(f); setText(f.content); setPath(p); setConflict(false); }
    catch (e) { toastError(e); }
  };
  useEffect(() => { loadTree(); }, [runsVersion]);
  useEffect(() => { loadFile(path, true); }, []);
  // an agent run may have changed the open file: reload it when the editor is clean
  useEffect(() => { if (file && !dirty) loadFile(path, true); }, [runsVersion]);
  useEffect(() => {
    const warn = (e) => { if (dirty) { e.preventDefault(); e.returnValue = ""; } };
    addEventListener("beforeunload", warn);
    return () => removeEventListener("beforeunload", warn);
  }, [dirty]);

  const save = async () => {
    if (!file || !dirty) return;
    setBusy(true);
    try { const f = await api("/api/skill/file", { method: "PUT", body: { path, content: text, sha: file.sha } }); setFile(f); toast("Đã lưu " + path); loadTree(); }
    catch (e) { if (e.status === 409) setConflict(true); else toastError(e); }
    finally { setBusy(false); }
  };
  const groups = {};
  for (const f of tree?.files || []) {
    const dir = f.path.includes("/") ? f.path.slice(0, f.path.lastIndexOf("/")) : "";
    (groups[dir] ||= []).push(f);
  }
  return html`<div class="page" style="max-width:none">
    <div class="page-head"><div><div class="eyebrow">Skill</div><h1>suno-que-huong</h1>
      <div class="muted small mono" style="margin-top:4px">${tree?.root}</div></div>
      <${ValidateButton} /></div>
    <div class="ide">
      <div class="card tree">${Object.keys(groups).sort().map((dir) => html`
        ${dir && html`<div class="tree-dir">${dir}</div>`}
        ${groups[dir].map((f) => html`<div class=${"tree-file" + (f.path === path ? " on" : "") + (f.editable ? "" : " dis")}
          title=${`${f.path} · ${bytes(f.size)} · ${ago(f.modified)}`} onClick=${() => f.editable && loadFile(f.path)}>
          <${Icon} name=${f.path.endsWith(".py") || f.path.endsWith(".sh") ? "code" : f.editable ? "file" : "image"} size=${14} />
          <span class="grow" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${f.path.split("/").pop()}</span></div>`)}`)}
      </div>
      <div class="card editor">
        <div class="editor-head"><${Icon} name="file" size=${14} /><span class="grow">${path}</span>
          ${dirty && html`<span class="pill warn">chưa lưu</span>`}
          <${Btn} size="sm" kind="primary" icon="save" busy=${busy} disabled=${!dirty} onClick=${save}>Lưu</${Btn}></div>
        <${CodeArea} value=${text} onChange=${setText} onSave=${save} />
      </div>
      <div class="side"><${AskAgent} path=${path} /><${SkillRuns} /></div>
    </div>
    ${conflict && html`<${Modal} title="File đã bị sửa ở nơi khác" onClose=${() => setConflict(false)} footer=${html`
        <${Btn} kind="ghost" onClick=${() => setConflict(false)}>Giữ bản đang sửa</${Btn}>
        <${Btn} kind="danger" icon="refresh" onClick=${() => loadFile(path, true)}>Tải bản mới, bỏ bản đang sửa</${Btn}>`}>
      <p>Có thể một agent vừa sửa <b>${path}</b>. Bản của bạn chưa được lưu để tránh ghi đè. Hãy chép phần cần giữ, rồi tải bản mới.</p>
    </${Modal}>`}
  </div>`;
}

function CodeArea({ value, onChange, onSave }) {
  const gutter = useRef(null);
  const lines = value.split("\n").length;
  const onKey = (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === "s") { e.preventDefault(); onSave(); return; }
    if (e.key === "Tab") {
      e.preventDefault();
      const el = e.currentTarget, s = el.selectionStart, end = el.selectionEnd;
      const next = value.slice(0, s) + "  " + value.slice(end);
      onChange(next);
      requestAnimationFrame(() => { el.selectionStart = el.selectionEnd = s + 2; });
    }
  };
  return html`<div class="code">
    <div class="gutter" ref=${gutter}>${Array.from({ length: lines }, (_, i) => html`<div>${i + 1}</div>`)}</div>
    <textarea spellcheck="false" value=${value} onInput=${(e) => onChange(e.target.value)} onKeyDown=${onKey}
      onScroll=${(e) => { if (gutter.current) gutter.current.scrollTop = e.currentTarget.scrollTop; }}></textarea>
  </div>`;
}

function AskAgent({ path }) {
  const ag = useAgentChoice();
  const [prompt, setPrompt] = useState("");
  const [busy, setBusy] = useState(false);
  const send = async () => {
    setBusy(true);
    try {
      const meta = await api("/api/runs", { method: "POST", body: { target: "skill", agent: ag.agent, model: ag.model, prompt } });
      openRun(meta.id); bumpRuns(); setPrompt(""); toast("Agent đang sửa skill");
    } catch (e) { toastError(e); } finally { setBusy(false); }
  };
  const samples = [
    "Thêm profile C: bolero trữ tình 70 BPM, giọng nam, đàn guitar phím lõm",
    `Rút gọn ${path} cho dễ đọc, giữ nguyên các quy tắc`,
    "Video: cho màu chữ karaoke vàng cam thay vì hồng",
  ];
  return html`<div class="card card-pad stack" style="gap:12px">
    <div class="row"><${Icon} name="bot" /><h3>Nhờ agent sửa skill</h3></div>
    <${AgentPicker} value=${ag.agent} onChange=${ag.setAgent} />
    <textarea class="textarea" rows="6" placeholder="Muốn skill thay đổi gì?" value=${prompt} onInput=${(e) => setPrompt(e.target.value)}
      onKeyDown=${(e) => (e.metaKey || e.ctrlKey) && e.key === "Enter" && prompt.trim() && send()}></textarea>
    <${PresetChips} value=${prompt} onChange=${setPrompt} />
    <div class="row"><span class="muted small grow"><span class="kbd">⌘↵</span> để gửi</span><${Btn} kind="primary" icon="send" busy=${busy} disabled=${!prompt.trim()} onClick=${send}>Gửi</${Btn}></div>
    <div class="stack" style="gap:6px">${samples.map((s) => html`<a class="small" style="color:var(--text-2);cursor:pointer" onClick=${() => setPrompt(s)}>→ ${s}</a>`)}</div>
    <div class="muted small">Agent làm việc ngay trong thư mục skill, theo luồng cập nhật của ak-skill-creator và chạy kiểm tra trước khi xong.</div>
  </div>`;
}

function ValidateButton() {
  const run = async () => {
    try { const meta = await api("/api/skill/validate", { method: "POST" }); openRun(meta.id); bumpRuns(); } catch (e) { toastError(e); }
  };
  return html`<${Btn} icon="shield" onClick=${run}>Kiểm tra skill</${Btn}>`;
}

function SkillRuns() {
  const [runs, setRuns] = useState([]);
  const runsVersion = useStore((s) => s.runsVersion);
  useEffect(() => { api("/api/runs?limit=60").then((r) => setRuns(r.filter((x) => x.step === "skill").slice(0, 8))).catch(() => {}); }, [runsVersion]);
  if (!runs.length) return null;
  return html`<div class="card"><div class="card-head"><h3>Lần sửa gần đây</h3></div>
    <table class="table runs-table"><tbody>${runs.map((r) => html`<tr onClick=${() => openRun(r.id)}>
      <td style="width:30px"><${AgentIcon} agent=${r.kind === "agent" ? r.agent : "script"} /></td>
      <td class="small">${r.title}<div class="muted">${ago(r.created)} · ${runDuration(r)}</div></td><td><${StatusPill} status=${r.status} /></td></tr>`)}</tbody></table></div>`;
}
