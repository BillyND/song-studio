// Settings: default agent, permissions, concurrency, CLI paths and models, image provider and keys, folders.
import { html, useState, useEffect, api, toast, toastError, store, useStore } from "./core.js";
import { Icon } from "./icons.js";
import { Btn, Seg, Field } from "./ui.js";

export function SettingsPage() {
  const boot = useStore((s) => s.boot);
  const [cfg, setCfg] = useState(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => { if (boot) setCfg(JSON.parse(JSON.stringify(boot.config))); }, [boot]);
  if (!cfg) return null;
  const set = (k, v) => setCfg((c) => ({ ...c, [k]: v }));
  const setAgent = (name, k, v) => setCfg((c) => ({ ...c, agents: { ...c.agents, [name]: { ...c.agents[name], [k]: v } } }));
  const save = async () => {
    setBusy(true);
    try {
      const r = await api("/api/settings", { method: "PUT", body: { ...cfg, maxConcurrent: Number(cfg.maxConcurrent) } });
      store.set({ boot: { ...boot, config: r.config, agents: r.agents } });
      toast("Đã lưu cài đặt");
    } catch (e) { toastError(e); } finally { setBusy(false); }
  };
  const infos = Object.fromEntries((boot.agents || []).map((a) => [a.name, a]));
  return html`<div class="page" style="max-width:860px">
    <div class="page-head"><div><div class="eyebrow">Studio</div><h1>Cài đặt</h1></div><${Btn} kind="primary" icon="save" busy=${busy} onClick=${save}>Lưu</${Btn}></div>
    <div class="stack" style="gap:18px">
      <div class="card card-pad stack">
        <h2>Agent</h2>
        <${Field} label="Agent mặc định"><${Seg} value=${cfg.defaultAgent} onChange=${(v) => set("defaultAgent", v)} options=${["claude", "omp", "codex"].map((n) => ({ value: n, label: n }))} /></${Field}>
        <${Field} label="Quyền của agent" hint="Agent chạy không có người duyệt nên không hỏi xin quyền được. 'Toàn quyền' giống cách bạn đang dùng (--dangerously-skip-permissions). 'Chỉ sửa file' an toàn hơn nhưng agent không chạy được script.">
          <${Seg} value=${cfg.permissionMode} onChange=${(v) => set("permissionMode", v)} options=${[{ value: "full", label: "Toàn quyền", icon: "zap" }, { value: "edits", label: "Chỉ sửa file", icon: "shield" }]} /></${Field}>
        <div class="form-row">
          <${Field} label="Việc nặng cùng lúc" hint="Tách giọng, canh lời, phân tích, render. Máy hay quá tải nên để 1.">
            <${Seg} value=${String(cfg.maxConcurrent)} onChange=${(v) => set("maxConcurrent", Number(v))} options=${["1", "2", "3"].map((n) => ({ value: n, label: n }))} /></${Field}>
          <${Field} label="Agent chạy song song" hint="Agent và tạo ảnh chủ yếu chờ API nên chạy song song được.">
            <${Seg} value=${String(cfg.maxAgents)} onChange=${(v) => set("maxAgents", Number(v))} options=${["1", "2", "3", "4", "6"].map((n) => ({ value: n, label: n }))} /></${Field}>
        </div>
      </div>
      <div class="card card-pad stack">
        <h2>Coding CLI</h2>
        ${["claude", "omp", "codex"].map((n) => html`<div class="row wrap" style="gap:12px;align-items:flex-end">
          <div style="width:90px"><b>${n}</b><div class="small ${infos[n]?.available ? "" : "muted"}" style=${{ color: infos[n]?.available ? "var(--ok)" : "" }}>${infos[n]?.available ? infos[n].version || "có" : "không thấy"}</div></div>
          <div class="grow"><${Field} label="Đường dẫn"><input class="input mono" value=${cfg.agents[n]?.bin} onInput=${(e) => setAgent(n, "bin", e.target.value)} /></${Field}></div>
          <div style="width:200px"><${Field} label="Model mặc định"><input class="input" placeholder="theo CLI" value=${cfg.agents[n]?.model} onInput=${(e) => setAgent(n, "model", e.target.value)} /></${Field}></div>
        </div>`)}
      </div>
      <div class="card card-pad stack">
        <h2>Bài mới</h2>
        <div class="form-row">${(cfg.credits || []).map((c, i) => html`
          <${Field} label=${"Ghi công " + (i + 1)}><div class="row"><input class="input" style="max-width:120px" value=${c[0]} onInput=${(e) => set("credits", cfg.credits.map((x, j) => (j === i ? [e.target.value, x[1]] : x)))} />
            <input class="input" placeholder="Tên" value=${c[1]} onInput=${(e) => set("credits", cfg.credits.map((x, j) => (j === i ? [x[0], e.target.value] : x)))} /></div></${Field}>`)}</div>
      </div>
      <${ImageCard} image=${cfg.image} onChange=${(v) => set("image", v)} />
      <div class="card card-pad stack">
        <div class="row"><h2 class="grow">Prompt mở rộng</h2>
          <${Btn} size="sm" icon="plus" onClick=${() => set("presets", [...(cfg.presets || []), { label: "", text: "" }])}>Thêm</${Btn}></div>
        <div class="muted small">Các câu dặn hay dùng. Chúng hiện thành nút dưới mọi ô dặn agent và ô yêu cầu mở rộng; bấm là chèn vào.</div>
        ${(cfg.presets || []).map((p, i) => html`<div class="row" style="align-items:flex-start">
          <input class="input" style="width:170px" placeholder="Tên nút" value=${p.label} onInput=${(e) => set("presets", cfg.presets.map((x, j) => (j === i ? { ...x, label: e.target.value } : x)))} />
          <textarea class="textarea" rows="2" style="min-height:42px" placeholder="Nội dung chèn vào" value=${p.text} onInput=${(e) => set("presets", cfg.presets.map((x, j) => (j === i ? { ...x, text: e.target.value } : x)))}></textarea>
          <${Btn} size="sm" kind="ghost" icon="x" title="Xoá" onClick=${() => set("presets", cfg.presets.filter((_, j) => j !== i))} /></div>`)}
      </div>
      <div class="card card-pad stack">
        <h2>Thư mục</h2>
        <${Field} label="Skill"><input class="input mono" value=${cfg.skillDir} onInput=${(e) => set("skillDir", e.target.value)} /></${Field}>
        <${Field} label="Các bài hát"><input class="input mono" value=${cfg.projectsDir} onInput=${(e) => set("projectsDir", e.target.value)} /></${Field}>
        <div class="muted small"><${Icon} name="shield" size=${12} /> Studio chỉ nghe ở <span class="mono">${cfg.addr}</span>, chặn truy cập từ trang web khác. Khoá API nằm trong Keychain của macOS và chỉ được đưa cho bước tạo ảnh.</div>
      </div>
    </div>
  </div>`;
}

const PROVIDERS = [
  { value: "hub", label: "Hub ảnh", hint: "Cổng OpenAI-compatible đang dùng (aihub). Model mặc định gpt-image-2.5." },
  { value: "openai", label: "OpenAI", hint: "Gọi thẳng OpenAI. Khoảng $0,85–1,1 cho 20 ảnh 2048×1152 chất lượng cao." },
  { value: "gemini", label: "Google Gemini", hint: "Nano Banana, khung 16:9 2K; giữ nhân vật đồng nhất tốt. Khoảng $1 cho 20 ảnh." },
];

// ImageCard picks the scene-image provider and manages its keys in the macOS keychain.
function ImageCard({ image, onChange }) {
  const [secrets, setSecrets] = useState(null);
  const [legacy, setLegacy] = useState(false);
  const load = () => api("/api/secrets").then((r) => { setSecrets(r.secrets); setLegacy(r.legacy); }).catch(toastError);
  useEffect(() => { load(); }, []);
  const set = (k, v) => onChange({ ...image, [k]: v });
  const p = PROVIDERS.find((x) => x.value === image.provider) || PROVIDERS[0];
  const info = (secrets || []).find((s) => s.name === "image." + image.provider);
  const hubSet = (secrets || []).find((s) => s.name === "image.hub")?.set;
  return html`<div class="card card-pad stack">
    <h2>Tạo ảnh</h2>
    <${Field} label="Nguồn ảnh" hint=${p.hint}>
      <${Seg} value=${image.provider} onChange=${(v) => set("provider", v)} options=${PROVIDERS.map((x) => ({ value: x.value, label: x.label }))} /></${Field}>
    <div class="form-row">
      <${Field} label="Model"><input class="input mono" value=${image.models[image.provider]} onInput=${(e) => set("models", { ...image.models, [image.provider]: e.target.value })} /></${Field}>
      ${image.provider !== "gemini" && html`<${Field} label="Kích thước" hint="16:9, ví dụ 2048x1152 hoặc 2560x1440">
        <input class="input mono" style="max-width:160px" value=${image.size} onInput=${(e) => set("size", e.target.value)} /></${Field}>`}
      ${image.provider !== "gemini" && html`<${Field} label="Chất lượng">
        <${Seg} value=${image.quality} onChange=${(v) => set("quality", v)} options=${[{ value: "medium", label: "Vừa" }, { value: "high", label: "Cao" }]} /></${Field}>`}
    </div>
    ${image.provider === "hub" && html`<${Field} label="Địa chỉ hub"><input class="input mono" value=${image.baseUrl} onInput=${(e) => set("baseUrl", e.target.value)} /></${Field}>`}
    <${KeyRow} name=${"image." + image.provider} label=${"Khoá " + p.label} info=${info} onSecrets=${setSecrets} />
    ${image.provider === "hub" && legacy && !hubSet && html`<div class="row wrap"><span class="muted small">Có khoá cũ trong <span class="mono">~/.config/suno-que-huong/aihub.key</span>.</span>
      <${Btn} size="sm" icon="upload" onClick=${() => api("/api/secrets/import-legacy", { method: "POST" }).then((r) => { setSecrets(r.secrets); toast("Đã chuyển khoá cũ vào Keychain"); }).catch(toastError)}>Chuyển vào Keychain</${Btn}></div>`}
    <div class="muted small"><${Icon} name="shield" size=${12} /> Khoá lưu trong Keychain của macOS, không nằm trong file cấu hình hay log, và chỉ được đưa cho bước tạo ảnh. Chương trình khác chạy dưới tài khoản máy của bạn vẫn có thể đọc Keychain, nên hãy dùng khoá có giới hạn chi tiêu. Nhớ bấm Lưu ở trên để lưu nguồn và model.</div>
  </div>`;
}

function KeyRow({ name, label, info, onSecrets }) {
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState("");
  const run = async (what, fn) => { setBusy(what); try { await fn(); } catch (e) { toastError(e); } finally { setBusy(""); } };
  const save = () => run("save", async () => { const r = await api(`/api/secrets/${name}`, { method: "PUT", body: { value } }); onSecrets(r.secrets); setValue(""); toast("Đã lưu khoá vào Keychain"); });
  const remove = () => confirm("Xoá khoá này khỏi Keychain?") && run("del", async () => { const r = await api(`/api/secrets/${name}`, { method: "DELETE" }); onSecrets(r.secrets); toast("Đã xoá khoá"); });
  const check = () => run("check", async () => { const r = await api(`/api/secrets/${name}/check`, { method: "POST" }); r.ok ? toast("Khoá dùng được") : toast("Khoá không dùng được: " + r.error, "bad"); });
  return html`<${Field} label=${label}>
    <div class="row wrap">
      <span class=${"pill " + (info?.set ? "ok" : "")}>${info?.set ? "Đã lưu " + (info.hint || "") : "Chưa có khoá"}</span>
      <input class="input mono grow" type="password" autocomplete="off" spellcheck="false" placeholder=${info?.set ? "Dán khoá mới để thay" : "Dán khoá API"} value=${value} onInput=${(e) => setValue(e.target.value)} />
      <${Btn} size="sm" kind="primary" icon="save" busy=${busy === "save"} disabled=${value.trim().length < 8} onClick=${save}>Lưu khoá</${Btn}>
      ${info?.set && html`<${Btn} size="sm" icon="check" busy=${busy === "check"} onClick=${check}>Kiểm tra</${Btn}>`}
      ${info?.set && html`<${Btn} size="sm" kind="ghost" icon="x" title="Xoá khoá" busy=${busy === "del"} onClick=${remove} />`}
    </div></${Field}>`;
}
