// Basic controls: buttons, status pills, switches, segmented choices, labelled fields.
import { html, t } from "../core.js";
import { Icon } from "../icons.js";

export const STATUS = {
  queued: { label: t("Đang chờ"), cls: "" },
  running: { label: t("Đang chạy"), cls: "run" },
  succeeded: { label: t("Hoàn thành"), cls: "ok" },
  failed: { label: t("Thất bại"), cls: "bad" },
  stopped: { label: t("Đã dừng"), cls: "warn" },
  interrupted: { label: t("Gián đoạn"), cls: "warn" },
  done: { label: t("Xong"), cls: "ok" },
  todo: { label: t("Chưa làm"), cls: "" },
  stale: { label: t("Cần làm lại"), cls: "warn" },
  skip: { label: t("Bỏ qua"), cls: "ok" },
};

export function StatusPill({ status, label }) {
  const s = STATUS[status] || { label: status, cls: "" };
  return html`<span class=${"pill " + s.cls}><span class="dot"></span>${label || s.label}</span>`;
}

export function Btn({ icon, children, kind = "", size = "", onClick, disabled, title, type = "button", busy }) {
  const cls = ["btn", kind, size, !children ? "icon" : ""].filter(Boolean).join(" ");
  return html`<button type=${type} class=${cls} onClick=${onClick} disabled=${disabled || busy} title=${title} aria-label=${title}>
    ${busy ? html`<${Icon} name="loader" class="spin" />` : icon ? html`<${Icon} name=${icon} />` : null}${children}
  </button>`;
}

export function Switch({ on, onChange, children }) {
  return html`<span class=${"switch" + (on ? " on" : "")} role="switch" aria-checked=${on} tabindex="0"
    onClick=${() => onChange(!on)} onKeyDown=${(e) => (e.key === " " || e.key === "Enter") && (e.preventDefault(), onChange(!on))}><i></i>${children}</span>`;
}

export function Seg({ value, options, onChange }) {
  return html`<div class="seg" role="radiogroup">${options.map((o) => html`
    <button type="button" class=${o.value === value ? "on" : ""} role="radio" aria-checked=${o.value === value} onClick=${() => onChange(o.value)}>
      ${o.icon && html`<${Icon} name=${o.icon} size=${13} />`}${o.label}</button>`)}</div>`;
}

export function Field({ label, children, hint }) {
  return html`<label class="field">${label}${children}${hint && html`<span class="muted small" style="font-weight:400">${hint}</span>`}</label>`;
}
