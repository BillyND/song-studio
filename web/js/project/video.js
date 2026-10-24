// Steps 4, 6 and 7: lyric alignment, preview frames and the karaoke video.
import { api, html, media, render, secs, t, theme, toast, toastError, useState } from "../core.js";
import { Btn, Empty, Field, Lightbox, Seg } from "../ui.js";
import { runAction, runPipeline, safeJSON } from "./actions.js";
import { ActionCard, ChainStatus } from "./parts.js";

export function AlignStep({ d, slug }) {
  return html`<div class="stack">
    <${ActionCard} title=${t("Canh lời từng chữ")} hint=${t("Lấy phụ đề Suno trong M4A, tách giọng bằng Demucs, Whisper canh từng chữ rồi khớp theo giọng thật")} onRun=${() => runAction(slug, "align")} label=${t("Canh lời")} />
    ${!d.audio && html`<div class="warn-box">${t("Cần thêm file nhạc ở bước 3 trước.")}</div>`}
    ${d.timings.length > 0 && html`<div class="scroll-x" style="max-height:460px"><table class="table"><thead><tr><th>#</th><th>${t("Bắt đầu")}</th><th>${t("Kết thúc")}</th><th>${t("Lệch Suno")}</th><th>${t("Lời")}</th></tr></thead><tbody>
      ${d.timings.map((l, i) => html`<tr><td class="num">${i + 1}</td><td class="num">${secs(l.start)}</td><td class="num">${secs(l.end)}</td>
        <td class="num">${Math.abs(l.drift) > 1 ? html`<span class="pill warn">${l.drift.toFixed(2)}s</span>` : l.drift.toFixed(2) + "s"}${l.source === "even" ? html` <span class="pill warn">${t("chia đều")}</span>` : ""}</td><td>${l.text}</td></tr>`)}
    </tbody></table></div>`}
  </div>`;
}

// t() runs at module load: the UI language is fixed per page load (changing it reloads the page).
const THEMES = [
  { value: "truyen-thong", label: t("Truyền thống"), hint: t("Chữ viết tay vàng cam, chữ chạy hồng. Hợp quê hương, thiếu nhi.") },
  { value: "hien-dai", label: t("Hiện đại"), hint: t("Chữ đậm trắng xanh, chữ chạy xanh. Hợp pop, ballad, rock, rap.") },
  { value: "co-dien", label: t("Cổ điển"), hint: t("Chữ có chân kem vàng, chữ chạy vàng. Hợp bolero, trữ tình, thánh ca.") },
];

export function StillsStep({ d, slug, stamp, reload }) {
  const [box, setBox] = useState(null);
  const [times, setTimes] = useState("");
  const cfg = safeJSON(d.config);
  const theme = THEMES.find((th) => th.value === cfg.video_theme) || THEMES[0];
  const setTheme = (v) => api(`/api/projects/${encodeURIComponent(slug)}/meta`, { method: "PATCH", body: { videoTheme: v } }).then(reload).catch(toastError);
  const parsed = times.split(/[\s,]+/).map(Number).filter((n) => isFinite(n) && n >= 0 && times.trim());
  return html`<div class="stack">
    <${Field} label=${t("Kiểu video")} hint=${t("{hint} Đổi kiểu rồi render lại khung để xem.", { hint: theme.hint })}>
      <${Seg} value=${theme.value} onChange=${setTheme} options=${THEMES.map((th) => ({ value: th.value, label: th.label }))} /></${Field}>
    <${ActionCard} title=${t("Render khung hình xem trước")} hint=${t("Mặc định: tựa đề, đếm ngược micro, giữa câu dài nhất, đoạn kết. Nhanh, vài giây mỗi khung")}
      onRun=${() => runAction(slug, "stills", parsed.length ? { times: parsed } : {})} label="Render" icon="eye">
      <input class="input" style="width:200px" placeholder=${t("giây tuỳ chọn: 8, 12.5, 90")} value=${times} onInput=${(e) => setTimes(e.target.value)} />
    </${ActionCard}>
    ${d.stills.length > 0 ? html`<div class="gallery">${d.stills.map((s) => html`<figure onClick=${() => setBox(s)}><img loading="lazy" decoding="async" src=${media(slug, s, stamp)} />
      <figcaption>${secs(parseFloat(s.replace(/.*still-/, "")))}</figcaption></figure>`)}</div>` : html`<${Empty} icon="eye" title=${t("Chưa có khung hình")}>${t("Render vài khung để kiểm tra chữ, micro và cảnh trước khi xuất cả video.")}</${Empty}>`}
    ${box && html`<${Lightbox} src=${media(slug, box, stamp)} caption=${box} onClose=${() => setBox(null)} />`}
  </div>`;
}

export function RenderStep({ d, slug, stamp, reload }) {
  const video = d.videos[d.videos.length - 1];
  const open = (reveal) => api(`/api/projects/${encodeURIComponent(slug)}/open`, { method: "POST", body: { path: video, reveal } }).catch(toastError);
  const trash = async () => {
    if (!confirm(t("Chuyển video \"{video}\" vào Thùng rác?\n\nBạn có thể xuất lại bất cứ lúc nào; bản đã chép sang Downloads (nếu có) không bị đụng tới.", { video }))) return;
    try { await api(`/api/projects/${encodeURIComponent(slug)}/video?path=${encodeURIComponent(video)}`, { method: "DELETE" }); toast(t("Đã chuyển video vào Thùng rác")); reload(); }
    catch (e) { toastError(e); }
  };
  return html`<div class="stack">
    ${video ? html`<video controls preload="metadata" src=${media(slug, video, stamp)}></video>
      <div class="row wrap"><b class="grow">${video}</b><${Btn} size="sm" icon="play" onClick=${() => open(false)}>${t("Mở bằng QuickTime")}</${Btn}><${Btn} size="sm" icon="folder" onClick=${() => open(true)}>${t("Hiện trong Finder")}</${Btn}><${Btn} size="sm" kind="danger" icon="trash" onClick=${trash}>${t("Xoá video")}</${Btn}></div>`
      : html`<${Empty} icon="film" title=${t("Chưa có video")}>${t("Xuất video sau khi đã xem trước khung hình.")}</${Empty}>`}
    <${ChainStatus} slug=${slug} steps=${["align", "scenes", "stills", "render", "sheet"]} title=${t("Chuỗi video")} />
    ${!d.audio && html`<div class="warn-box">${t("Chưa có nhạc. Thêm file M4A từ Suno ở")} <a href=${`#/p/${slug}/audio`} style="color:inherit;text-decoration:underline">${t("bước 3")}</a> ${t("trước.")}</div>`}
    <${ActionCard} agent title=${t("Làm video tự động")} hint=${t("Canh lời → agent lên cảnh → tạo ảnh còn thiếu → khung xem trước → xuất video, kèm bản nhạc. Canh lời và lên cảnh được bỏ qua khi đã làm và vẫn còn đúng")} busy=${d.steps.some((s) => s.status === "running")} onRun=${() => runPipeline(slug, "video")} label=${t("Chạy cả chuỗi")} icon="zap" />
    <${ActionCard} title=${t("Chỉ xuất video")} hint=${t("1920×1080, 30 fps, H.264 + AAC. Dựng trên nhiều nhân CPU cùng lúc, khoảng 1–2 phút cho bài 4 phút")} onRun=${() => runAction(slug, "render")} label=${t("Xuất video")} icon="film" />
  </div>`;
}
