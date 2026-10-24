// Server calls for a song: run a step or a chain, save files, move to Trash, read settings.
import { api, bumpRuns, go, openRun, store, t, toast, toastError, useState } from "../core.js";

export const stepDone = (d, id) => ["done", "skip"].includes(d.steps.find((s) => s.id === id)?.status);

export const goStep = (slug, id) => go(`/p/${encodeURIComponent(slug)}/${id}`);

export async function runAction(slug, action, body = {}) {
  try {
    const meta = await api(`/api/projects/${encodeURIComponent(slug)}/actions/${action}`, { method: "POST", body });
    openRun(meta.id);
    bumpRuns();
    toast(meta.status === "queued" ? t("Đã xếp hàng, sẽ chạy khi tới lượt") : t("Đã bắt đầu"));
    return meta;
  } catch (e) { toastError(e); }
}

const pending = new Set(); // chains being submitted, so a double click queues nothing twice

export async function runPipeline(slug, name, body = {}) {
  const key = slug + "/" + name;
  if (pending.has(key)) return;
  pending.add(key);
  try {
    const metas = await api(`/api/projects/${encodeURIComponent(slug)}/pipeline/${name}`, { method: "POST", body });
    openRun((metas.find((m) => !(m.after || []).length) || metas[0]).id);
    bumpRuns();
    toast(t("Đã xếp {n} việc, các việc độc lập chạy song song", { n: metas.length }));
    return metas;
  } catch (e) { toastError(e); } finally { pending.delete(key); }
}

// useSave returns [busy, save]; save resolves true only when the file was written.
export function useSave(slug, reload) {
  const [busy, setBusy] = useState(false);
  return [busy, async (path, content, sha) => {
    setBusy(true);
    try { await api(`/api/projects/${encodeURIComponent(slug)}/files`, { method: "PUT", body: { path, content, sha } }); toast(t("Đã lưu")); await reload(); return true; }
    catch (e) { toastError(e.status === 409 ? new Error(t("project.json vừa được sửa ở nơi khác (có thể bởi agent). Đóng và mở lại để lấy bản mới.")) : e); return false; }
    finally { setBusy(false); }
  }];
}

// ---------- steps ----------

// imageSource names the provider and model chosen in Settings, for step hints.
export function imageSource() {
  const im = store.get().boot?.config?.image;
  if (!im) return t("Nguồn ảnh ở Cài đặt");
  return `${{ hub: "OpenAI-compatible", openai: "OpenAI", gemini: "Google Gemini" }[im.provider] || im.provider} · ${im.models?.[im.provider] || ""}`;
}

// Deletes move files to the macOS Trash, so a slip can be undone with Finder's "Put Back".
export async function trashProject(d, slug) {
  const name = [d.title, d.subtitle].filter(Boolean).join(" – ");
  if (!confirm(t("Chuyển cả bài \"{name}\" vào Thùng rác?\n\nThư mục {dir} (lời, nhạc, ảnh, video) sẽ nằm trong Thùng rác, lấy lại được bằng Finder cho tới khi bạn dọn Thùng rác.", { name, dir: d.dir }))) return;
  try {
    await api(`/api/projects/${encodeURIComponent(slug)}`, { method: "DELETE" });
    toast(t("Đã chuyển bài vào Thùng rác"));
    go("/");
  } catch (e) { toastError(e); }
}

export function safeJSON(raw) {
  if (raw && typeof raw === "object") return raw;
  try { return JSON.parse(raw || "{}"); } catch { return {}; }
}
