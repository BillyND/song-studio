// Formatting for the UI: relative times, durations, sizes, media URLs, copying text.
import { t, toast } from "../core.js";

export function ago(iso) {
  if (!iso || iso.startsWith("0001")) return "";
  const s = (Date.now() - new Date(iso).getTime()) / 1000;
  if (s < 45) return t("vừa xong");
  if (s < 3600) return t("{n} phút trước", { n: Math.round(s / 60) });
  if (s < 86400) return t("{n} giờ trước", { n: Math.round(s / 3600) });
  return t("{n} ngày trước", { n: Math.round(s / 86400) });
}
export function dur(ms) {
  if (ms < 0 || !isFinite(ms)) return "";
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${String(s % 60).padStart(2, "0")}s`;
  return `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`;
}
export function runDuration(r) {
  const zero = (t) => !t || t.startsWith("0001");
  if (zero(r.started)) return "";
  const end = zero(r.ended) ? Date.now() : new Date(r.ended).getTime();
  return dur(end - new Date(r.started).getTime());
}
export function secs(t) {
  if (t == null || !isFinite(t)) return "";
  const m = Math.floor(t / 60);
  return `${m}:${(t - m * 60).toFixed(1).padStart(4, "0")}`;
}
export function bytes(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1 << 20) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / (1 << 20)).toFixed(1)} MB`;
}
export const media = (slug, rel, bust) => `/media/${encodeURIComponent(slug)}/${rel.split("/").map(encodeURIComponent).join("/")}${bust ? `?v=${bust}` : ""}`;

export async function copyText(text) {
  try { await navigator.clipboard.writeText(text); toast(t("Đã chép vào clipboard")); }
  catch { toast(t("Không chép được, hãy chọn và chép tay"), "bad"); }
}

export const isVideo = (name) => /\.(mp4|webm|mov|mkv)$/i.test(name);
