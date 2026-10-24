// Settings: language, agents and their models, concurrency, new-song defaults, folders.
import { api, html, render, store, t, toast, toastError, useEffect, useState, useStore } from "../core.js";
import { Icon } from "../icons.js";
import { Btn, Field, Seg } from "../ui.js";
import { DoctorCard } from "./doctor.js";
import { ImageCard } from "./image.js";

// agent steps whose model can be set apart from the agent's default
const STEP_MODELS = [["lyrics.agent", "Soát lời"], ["style.agent", "Phối khí & Styles"], ["scenes.agent", "Lên cảnh"], ["suno.agent", "Điền form Suno"]];

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
      if (r.config.language !== boot.config.language) { location.reload(); return; } // every text redraws in the new language
      store.set({ boot: { ...boot, config: r.config, agents: r.agents } });
      toast(t("Đã lưu cài đặt"));
    } catch (e) { toastError(e); } finally { setBusy(false); }
  };
  const infos = Object.fromEntries((boot.agents || []).map((a) => [a.name, a]));
  return html`<div class="page" style="max-width:860px">
    <div class="page-head"><div><div class="eyebrow">Studio</div><h1>${t("Cài đặt")}</h1></div><${Btn} kind="primary" icon="save" busy=${busy} onClick=${save}>${t("Lưu")}</${Btn}></div>
    <div class="stack" style="gap:18px">
      <div class="card card-pad stack">
        <h2>Ngôn ngữ · Language</h2>
        <${Field} label="Ngôn ngữ giao diện · Interface language" hint="Lưu để áp dụng. Lời bài hát và ngôn ngữ hát của từng bài không đổi. · Save to apply; lyrics and each song's sung language stay as they are.">
          <${Seg} value=${cfg.language || "vi"} onChange=${(v) => set("language", v)} options=${[{ value: "vi", label: "Tiếng Việt" }, { value: "en", label: "English" }]} /></${Field}>
      </div>
      <${DoctorCard} />
      <div class="card card-pad stack">
        <h2>Agent</h2>
        <${Field} label=${t("Agent mặc định")}><${Seg} value=${cfg.defaultAgent} onChange=${(v) => set("defaultAgent", v)} options=${["claude", "omp", "codex"].map((n) => ({ value: n, label: n }))} /></${Field}>
        <${Field} label=${t("Quyền của agent")} hint=${t("Agent chạy không có người duyệt nên không hỏi xin quyền được. 'Toàn quyền' giống cách bạn đang dùng (--dangerously-skip-permissions). 'Chỉ sửa file' an toàn hơn nhưng agent không chạy được script.")}>
          <${Seg} value=${cfg.permissionMode} onChange=${(v) => set("permissionMode", v)} options=${[{ value: "full", label: t("Toàn quyền"), icon: "zap" }, { value: "edits", label: t("Chỉ sửa file"), icon: "shield" }]} /></${Field}>
        <div class="form-row">
          <${Field} label=${t("Việc nặng cùng lúc")} hint=${t("Tách giọng, canh lời, phân tích, render. Máy hay quá tải nên để 1.")}>
            <${Seg} value=${String(cfg.maxConcurrent)} onChange=${(v) => set("maxConcurrent", Number(v))} options=${["1", "2", "3"].map((n) => ({ value: n, label: n }))} /></${Field}>
          <${Field} label=${t("Agent chạy song song")} hint=${t("Agent và tạo ảnh chủ yếu chờ API nên chạy song song được.")}>
            <${Seg} value=${String(cfg.maxAgents)} onChange=${(v) => set("maxAgents", Number(v))} options=${["1", "2", "3", "4", "6"].map((n) => ({ value: n, label: n }))} /></${Field}>
        </div>
      </div>
      <div class="card card-pad stack">
        <h2>Coding CLI</h2>
        ${["claude", "omp", "codex"].map((n) => html`<div class="row wrap" style="gap:12px;align-items:flex-end">
          <div style="width:90px"><b>${n}</b><div class="small ${infos[n]?.available ? "" : "muted"}" style=${{ color: infos[n]?.available ? "var(--ok)" : "" }}>${infos[n]?.available ? infos[n].version || t("có") : t("không thấy")}</div></div>
          <div class="grow"><${Field} label=${t("Đường dẫn")}><input class="input mono" value=${cfg.agents[n]?.bin} onInput=${(e) => setAgent(n, "bin", e.target.value)} /></${Field}></div>
          <div style="width:200px"><${Field} label=${t("Model mặc định")}><input class="input" placeholder=${t("theo CLI")} value=${cfg.agents[n]?.model} onInput=${(e) => setAgent(n, "model", e.target.value)} /></${Field}></div>
          <div class="step-models">${STEP_MODELS.map(([id, label]) => html`<${Field} label=${t(label)}>
            <input class="input" placeholder=${t("như mặc định")} value=${cfg.agents[n]?.steps?.[id] || ""}
              onInput=${(e) => setAgent(n, "steps", { ...(cfg.agents[n]?.steps || {}), [id]: e.target.value.trim() })} /></${Field}>`)}</div>
        </div>`)}
        <div class="muted small">${t("Model theo bước: bước nhẹ chạy model nhanh, rẻ hơn (ví dụ sonnet). Để trống là dùng model mặc định. Model bạn chọn lúc giao việc luôn được ưu tiên.")}</div>
      </div>
      <div class="card card-pad stack">
        <h2>${t("Bài mới")}</h2>
        <div class="form-row">${(cfg.credits || []).map((c, i) => html`
          <${Field} label=${t("Ghi công {n}", { n: i + 1 })}><div class="row"><input class="input" style="max-width:120px" value=${c[0]} onInput=${(e) => set("credits", cfg.credits.map((x, j) => (j === i ? [e.target.value, x[1]] : x)))} />
            <input class="input" placeholder=${t("Tên")} value=${c[1]} onInput=${(e) => set("credits", cfg.credits.map((x, j) => (j === i ? [x[0], e.target.value] : x)))} /></div></${Field}>`)}</div>
      </div>
      <${ImageCard} image=${cfg.image} onChange=${(v) => set("image", v)} />
      <div class="card card-pad stack">
        <div class="row"><h2 class="grow">${t("Prompt mở rộng")}</h2>
          <${Btn} size="sm" icon="plus" onClick=${() => set("presets", [...(cfg.presets || []), { label: "", text: "" }])}>${t("Thêm")}</${Btn}></div>
        <div class="muted small">${t("Các câu dặn hay dùng. Chúng hiện thành nút dưới mọi ô dặn agent và ô yêu cầu mở rộng; bấm là chèn vào.")}</div>
        ${(cfg.presets || []).map((p, i) => html`<div class="row" style="align-items:flex-start">
          <input class="input" style="width:170px" placeholder=${t("Tên nút")} value=${p.label} onInput=${(e) => set("presets", cfg.presets.map((x, j) => (j === i ? { ...x, label: e.target.value } : x)))} />
          <textarea class="textarea" rows="2" style="min-height:42px" placeholder=${t("Nội dung chèn vào")} value=${p.text} onInput=${(e) => set("presets", cfg.presets.map((x, j) => (j === i ? { ...x, text: e.target.value } : x)))}></textarea>
          <${Btn} size="sm" kind="ghost" icon="x" title=${t("Xoá")} onClick=${() => set("presets", cfg.presets.filter((_, j) => j !== i))} /></div>`)}
      </div>
      <div class="card card-pad stack">
        <h2>${t("Thư mục")}</h2>
        <${Field} label="Skill"><input class="input mono" value=${cfg.skillDir} onInput=${(e) => set("skillDir", e.target.value)} /></${Field}>
        <${Field} label=${t("Các bài hát")}><input class="input mono" value=${cfg.projectsDir} onInput=${(e) => set("projectsDir", e.target.value)} /></${Field}>
        <div class="muted small"><${Icon} name="shield" size=${12} /> ${t("Studio chỉ nghe ở")} <span class="mono">${cfg.addr}</span>${t(", chặn truy cập từ trang web khác. Khoá API nằm trong Keychain của macOS và chỉ được đưa cho bước tạo ảnh.")}</div>
      </div>
    </div>
  </div>`;
}
