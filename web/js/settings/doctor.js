// Machine check: tools the studio needs, with how to fix what is missing.
import { api, html, t, toastError, useEffect, useState } from "../core.js";
import { Icon } from "../icons.js";
import { Btn } from "../ui.js";

// DoctorCard lists what the studio needs on this Mac and how to fix what is missing.
export function DoctorCard() {
  const [checks, setChecks] = useState(null);
  const [busy, setBusy] = useState(false);
  const load = async () => { setBusy(true); try { setChecks(await api("/api/doctor")); } catch (e) { toastError(e); } finally { setBusy(false); } };
  useEffect(() => { load(); }, []);
  const bad = (checks || []).filter((c) => c.status !== "ok").length;
  const icon = { ok: "check", warn: "alert", bad: "x" };
  return html`<div class="card card-pad stack">
    <div class="row wrap"><h2 class="grow">${t("Kiểm tra máy")}</h2>
      ${checks && html`<span class=${"pill " + (bad ? "warn" : "ok")}>${bad ? t("{n} mục cần xem", { n: bad }) : t("Đủ cả")}</span>`}
      <${Btn} size="sm" icon="refresh" busy=${busy} onClick=${load}>${t("Kiểm tra lại")}</${Btn}>
      <a class="btn sm" href="/api/diagnostics" download><${Icon} name="save" />${t("Xuất gói chẩn đoán")}</a></div>
    ${checks ? html`<div class="doctor">${checks.map((c) => html`<div class=${"doctor-row " + c.status}>
      <span class="doctor-icon"><${Icon} name=${icon[c.status]} size=${14} /></span>
      <div class="grow" style="min-width:0"><b>${c.label}</b><div class="muted small ellipsis" title=${c.detail}>${c.detail}</div></div>
      ${c.fix && html`<div class="small doctor-fix">${c.fix}</div>`}</div>`)}</div>` : html`<div class="muted small">${t("Đang kiểm tra…")}</div>`}
    <div class="muted small">${t("Gói chẩn đoán gồm kết quả kiểm tra, cài đặt, nhật ký studio và các lượt chạy gần đây; khoá API đã được che. Gửi kèm khi báo lỗi.")}</div>
  </div>`;
}
