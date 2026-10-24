// Settings: default agent, permissions, concurrency, CLI paths and models, folders.
import { html, useState, useEffect, api, toast, toastError, store, useStore } from "./core.js";
import { Icon } from "./icons.js";
import { Btn, Seg, Field } from "./ui.js";

export function SettingsPage() {
  const boot = useStore((s) => s.boot);
  const [cfg, setCfg] = useState(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => { if (boot) setCfg(JSON.parse(JSON.stringify(boot.config))); }, [boot]);
  if (!cfg) return null;
  const set = (k, v) => setCfg({ ...cfg, [k]: v });
  const setAgent = (name, k, v) => setCfg({ ...cfg, agents: { ...cfg.agents, [name]: { ...cfg.agents[name], [k]: v } } });
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
        <${Field} label="Số việc chạy cùng lúc" hint="Máy hay bị quá tải, nên mặc định 1: việc khác xếp hàng chờ.">
          <${Seg} value=${String(cfg.maxConcurrent)} onChange=${(v) => set("maxConcurrent", Number(v))} options=${["1", "2", "3"].map((n) => ({ value: n, label: n }))} /></${Field}>
      </div>
      <div class="card card-pad stack">
        <h2>Coding CLI</h2>
        ${["claude", "omp", "codex"].map((n) => html`<div class="row" style="gap:12px;align-items:flex-end">
          <div style="width:90px"><b>${n}</b><div class="small ${infos[n]?.available ? "" : "muted"}" style=${{ color: infos[n]?.available ? "var(--ok)" : "" }}>${infos[n]?.available ? infos[n].version || "có" : "không thấy"}</div></div>
          <div class="grow"><${Field} label="Đường dẫn"><input class="input mono" value=${cfg.agents[n]?.bin} onInput=${(e) => setAgent(n, "bin", e.target.value)} /></${Field}></div>
          <div style="width:200px"><${Field} label="Model mặc định"><input class="input" placeholder="theo CLI" value=${cfg.agents[n]?.model} onInput=${(e) => setAgent(n, "model", e.target.value)} /></${Field}></div>
        </div>`)}
      </div>
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
        <div class="muted small"><${Icon} name="shield" size=${12} /> Studio chỉ nghe ở <span class="mono">${cfg.addr}</span>, chặn truy cập từ trang web khác. Khoá API ảnh đọc từ <span class="mono">~/.config/suno-que-huong/aihub.key</span> và không bao giờ hiện ở đây.</div>
      </div>
    </div>
  </div>`;
}
