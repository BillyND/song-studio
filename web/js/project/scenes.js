// Step 5: scene list, AI images and the project.json editor.
import { html, media, secs, t, useState } from "../core.js";
import { Btn, Lightbox, Modal } from "../ui.js";
import { imageSource, runAction, safeJSON, useSave } from "./actions.js";
import { ActionCard, AgentAction } from "./parts.js";

export function ScenesStep({ d, slug, reload, stamp }) {
  const [box, setBox] = useState(null);
  const [editing, setEditing] = useState(false);
  const cfg = safeJSON(d.config);
  const scenes = cfg.scenes || [];
  const imgs = new Set(d.images);
  return html`<div class="stack">
    <${AgentAction} slug=${slug} action="scenes.agent" title=${t("Lên danh sách cảnh theo lời")} hint=${t("Ghi scenes vào project.json: mỗi cảnh đổi ở đầu một câu, prompt tiếng Anh nêu đúng địa danh")} />
    <${ActionCard} title=${t("Tạo ảnh còn thiếu")} hint=${t("{source} · 4 ảnh song song · ảnh đã có được giữ, xoá file để tạo lại", { source: imageSource() })} onRun=${() => runAction(slug, "images")} label=${t("Tạo ảnh")} icon="image" />
    <div class="row"><h3>${t("{n} cảnh", { n: scenes.length })}</h3><span class="spacer"></span><${Btn} size="sm" icon="code" onClick=${() => setEditing(true)}>${t("Sửa project.json")}</${Btn}></div>
    ${scenes.length > 0 && html`<div class="scroll-x" style="max-height:380px"><table class="table"><thead><tr><th>${t("Bắt đầu")}</th><th>${t("Cảnh")}</th><th>Prompt</th><th></th></tr></thead><tbody>
      ${scenes.map((s) => { const img = `img/${s.id}.png`; return html`<tr><td class="num">${secs(s.start)}</td><td class="mono small">${s.id}</td>
        <td class="small">${s.reuse ? html`<span class="muted">${t("dùng lại ảnh tựa đề")}</span>` : (s.prompt || "").slice(0, 160)}</td>
        <td>${imgs.has(img) ? html`<img loading="lazy" decoding="async" src=${media(slug, img, stamp)} alt="" style="width:72px;border-radius:6px;cursor:zoom-in" onClick=${() => setBox(img)} />` : s.reuse ? "" : html`<span class="pill">${t("chưa có")}</span>`}</td></tr>`; })}
    </tbody></table></div>`}
    ${d.images.length > 0 && html`<div class="gallery">${d.images.map((img) => html`<figure onClick=${() => setBox(img)}><img loading="lazy" decoding="async" src=${media(slug, img, stamp)} /><figcaption>${img.replace("img/", "")}</figcaption></figure>`)}</div>`}
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
      <${Btn} kind="ghost" onClick=${onClose}>${t("Đóng")}</${Btn}><${Btn} kind="primary" icon="save" busy=${busy} disabled=${!!bad} onClick=${async () => { await save("project.json", val, d.configSha); onClose(); }}>${t("Lưu")}</${Btn}>`}>
    <textarea class="textarea mono" style="min-height:60vh" value=${val} onInput=${(e) => setVal(e.target.value)}></textarea>
  </${Modal}>`;
}
