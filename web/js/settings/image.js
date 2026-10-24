// Image provider, model and API keys (kept in the macOS Keychain).
import { api, html, t, toast, toastError, useEffect, useState } from "../core.js";
import { Icon } from "../icons.js";
import { Btn, Field, Seg } from "../ui.js";

const PROVIDERS = [
  { value: "hub", label: "OpenAI-compatible", hint: t("Bất kỳ dịch vụ nào có API ảnh kiểu OpenAI (aihub, OpenRouter, LiteLLM…): điền địa chỉ, model và khoá. Mặc định là aihub với gpt-image-2.5.") },
  { value: "openai", label: "OpenAI", hint: t("Gọi thẳng OpenAI. Khoảng $0,85–1,1 cho 20 ảnh 2048×1152 chất lượng cao.") },
  { value: "gemini", label: "Google Gemini", hint: t("Nano Banana, khung 16:9 2K; giữ nhân vật đồng nhất tốt. Khoảng $1 cho 20 ảnh.") },
];

// ImageCard picks the scene-image provider and manages its keys in the macOS keychain.
export function ImageCard({ image, onChange }) {
  const [secrets, setSecrets] = useState(null);
  const [legacy, setLegacy] = useState(false);
  const load = () => api("/api/secrets").then((r) => { setSecrets(r.secrets); setLegacy(r.legacy); }).catch(toastError);
  useEffect(() => { load(); }, []);
  const set = (k, v) => onChange({ ...image, [k]: v });
  const p = PROVIDERS.find((x) => x.value === image.provider) || PROVIDERS[0];
  const info = (secrets || []).find((s) => s.name === "image." + image.provider);
  const hubSet = (secrets || []).find((s) => s.name === "image.hub")?.set;
  return html`<div class="card card-pad stack">
    <h2>${t("Tạo ảnh AI")}</h2>
    <${Field} label=${t("Nguồn ảnh")} hint=${p.hint}>
      <${Seg} value=${image.provider} onChange=${(v) => set("provider", v)} options=${PROVIDERS.map((x) => ({ value: x.value, label: x.label }))} /></${Field}>
    <div class="form-row">
      <${Field} label="Model"><input class="input mono" value=${image.models[image.provider]} onInput=${(e) => set("models", { ...image.models, [image.provider]: e.target.value })} /></${Field}>
      ${image.provider !== "gemini" && html`<${Field} label=${t("Kích thước")} hint=${t("16:9, ví dụ 2048x1152 hoặc 2560x1440")}>
        <input class="input mono" style="max-width:160px" value=${image.size} onInput=${(e) => set("size", e.target.value)} /></${Field}>`}
      ${image.provider !== "gemini" && html`<${Field} label=${t("Chất lượng")}>
        <${Seg} value=${image.quality} onChange=${(v) => set("quality", v)} options=${[{ value: "medium", label: t("Vừa") }, { value: "high", label: t("Cao") }]} /></${Field}>`}
    </div>
    ${image.provider === "hub" && html`<${Field} label=${t("Địa chỉ API (base URL)")} hint=${t("Ví dụ https://api.openai.com/v1, https://openrouter.ai/api/v1")}><input class="input mono" value=${image.baseUrl} onInput=${(e) => set("baseUrl", e.target.value)} /></${Field}>`}
    <${KeyRow} name=${"image." + image.provider} label=${t("Khoá {provider}", { provider: p.label })} info=${info} onSecrets=${setSecrets} />
    ${image.provider === "hub" && legacy && !hubSet && html`<div class="row wrap"><span class="muted small">${t("Có khoá cũ trong")} <span class="mono">~/.config/suno-que-huong/aihub.key</span>.</span>
      <${Btn} size="sm" icon="upload" onClick=${() => api("/api/secrets/import-legacy", { method: "POST" }).then((r) => { setSecrets(r.secrets); toast(t("Đã chuyển khoá cũ vào Keychain")); }).catch(toastError)}>${t("Chuyển vào Keychain")}</${Btn}></div>`}
    <div class="muted small"><${Icon} name="shield" size=${12} /> ${t("Khoá lưu trong Keychain của macOS, không nằm trong file cấu hình hay log, và chỉ được đưa cho bước tạo ảnh. Chương trình khác chạy dưới tài khoản máy của bạn vẫn có thể đọc Keychain, nên hãy dùng khoá có giới hạn chi tiêu. Nhớ bấm Lưu ở trên để lưu nguồn và model.")}</div>
  </div>`;
}

function KeyRow({ name, label, info, onSecrets }) {
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState("");
  const run = async (what, fn) => { setBusy(what); try { await fn(); } catch (e) { toastError(e); } finally { setBusy(""); } };
  const save = () => run("save", async () => { const r = await api(`/api/secrets/${name}`, { method: "PUT", body: { value } }); onSecrets(r.secrets); setValue(""); toast(t("Đã lưu khoá vào Keychain")); });
  const remove = () => confirm(t("Xoá khoá này khỏi Keychain?")) && run("del", async () => { const r = await api(`/api/secrets/${name}`, { method: "DELETE" }); onSecrets(r.secrets); toast(t("Đã xoá khoá")); });
  const check = () => run("check", async () => { const r = await api(`/api/secrets/${name}/check`, { method: "POST" }); r.ok ? toast(t("Khoá dùng được")) : toast(t("Khoá không dùng được: {error}", { error: r.error }), "bad"); });
  return html`<${Field} label=${label}>
    <div class="row wrap">
      <span class=${"pill " + (info?.set ? "ok" : "")}>${info?.set ? t("Đã lưu {hint}", { hint: info.hint || "" }) : t("Chưa có khoá")}</span>
      <input class="input mono grow" type="password" autocomplete="off" spellcheck="false" placeholder=${info?.set ? t("Dán khoá mới để thay") : t("Dán khoá API")} value=${value} onInput=${(e) => setValue(e.target.value)} />
      <${Btn} size="sm" kind="primary" icon="save" busy=${busy === "save"} disabled=${value.trim().length < 8} onClick=${save}>${t("Lưu khoá")}</${Btn}>
      ${info?.set && html`<${Btn} size="sm" icon="check" busy=${busy === "check"} onClick=${check}>${t("Kiểm tra")}</${Btn}>`}
      ${info?.set && html`<${Btn} size="sm" kind="ghost" icon="x" title=${t("Xoá khoá")} busy=${busy === "del"} onClick=${remove} />`}
    </div></${Field}>`;
}
