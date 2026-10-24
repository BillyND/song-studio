// Shared runtime: Preact bindings, API client, a tiny global store and formatting helpers.
import { h, render } from "/lib/preact.mjs";
import { useState, useEffect, useRef, useMemo, useCallback, useLayoutEffect, useErrorBoundary } from "/lib/hooks.mjs";
import htm from "/lib/htm.mjs";

export const html = htm.bind(h);
export { h, render, useState, useEffect, useRef, useMemo, useCallback, useLayoutEffect, useErrorBoundary };

const token = document.querySelector('meta[name="studio-token"]')?.content || "";

export class ApiError extends Error {
  constructor(message, status) { super(message); this.status = status; }
}

export async function api(path, { method = "GET", body, raw } = {}) {
  const opts = { method, headers: {} };
  if (method !== "GET") opts.headers["X-Studio-Token"] = token;
  if (body instanceof FormData) opts.body = body;
  else if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  if (raw) return res;
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  if (!res.ok) throw new ApiError((data && data.error) || text || res.statusText, res.status);
  return data;
}

// ---------- store ----------
const state = {
  boot: null,
  system: null,
  consoleRun: null, // id of the run shown in the drawer
  toasts: [],
  runsVersion: 0, // bumps whenever a run starts or ends so views can refetch
  theme: "", // the current theme, so the toggle re-renders the moment it changes
};
const listeners = new Set();
export const store = {
  get: () => state,
  set(patch) {
    Object.assign(state, typeof patch === "function" ? patch(state) : patch);
    listeners.forEach((fn) => fn(state));
  },
  subscribe(fn) { listeners.add(fn); return () => listeners.delete(fn); },
};
export function useStore(select = (s) => s) {
  const [, force] = useState(0);
  const sel = useRef(select);
  sel.current = select;
  const last = useRef(select(state));
  useEffect(() => store.subscribe((s) => {
    const next = sel.current(s);
    if (next !== last.current) { last.current = next; force((n) => n + 1); }
  }), []);
  return select(state);
}

let toastId = 0;
export function toast(message, kind = "ok") {
  const id = ++toastId;
  store.set((s) => ({ toasts: [...s.toasts, { id, message, kind }] }));
  setTimeout(() => store.set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })), kind === "bad" ? 7000 : 3500);
}
export function toastError(err) { toast(err?.message || String(err), "bad"); }

export function openRun(id) { store.set({ consoleRun: id }); }
export function bumpRuns() { store.set((s) => ({ runsVersion: s.runsVersion + 1 })); }

// ---------- routing (hash based, no server routes needed) ----------
export function useRoute() {
  const [hash, setHash] = useState(location.hash || "#/");
  useEffect(() => {
    const on = () => setHash(location.hash || "#/");
    addEventListener("hashchange", on);
    return () => removeEventListener("hashchange", on);
  }, []);
  return hash.replace(/^#/, "").split("/").filter(Boolean).map(decodeURIComponent);
}
export const go = (path) => { location.hash = path; };

// ---------- formatting ----------
export function ago(iso) {
  if (!iso || iso.startsWith("0001")) return "";
  const s = (Date.now() - new Date(iso).getTime()) / 1000;
  if (s < 45) return "vừa xong";
  if (s < 3600) return `${Math.round(s / 60)} phút trước`;
  if (s < 86400) return `${Math.round(s / 3600)} giờ trước`;
  return `${Math.round(s / 86400)} ngày trước`;
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
  try { await navigator.clipboard.writeText(text); toast("Đã chép vào clipboard"); }
  catch { toast("Không chép được, hãy chọn và chép tay", "bad"); }
}

export const theme = {
  get: () => localStorageGet("theme") || "dark",
  // Switching themes restyles every element. Per-element colour transitions made that look like
  // lag (each part faded at its own pace), so they are paused for the switch, and where the
  // engine supports it the whole page crossfades once instead.
  set(v) {
    const root = document.documentElement;
    if (store.get().theme !== v) store.set({ theme: v }); // the icon flips now, even while the page crossfades
    if (root.dataset.theme === v) return;
    const apply = () => {
      root.classList.add("theme-switching");
      root.dataset.theme = v;
      localStorageSet("theme", v);
      requestAnimationFrame(() => requestAnimationFrame(() => root.classList.remove("theme-switching")));
    };
    const calm = matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (document.startViewTransition && !calm && root.dataset.theme) document.startViewTransition(apply);
    else apply();
  },
};
export function localStorageGet(k) { try { return localStorage.getItem("qhs." + k); } catch { return null; } }
export function localStorageSet(k, v) { try { localStorage.setItem("qhs." + k, v); } catch { /* private mode */ } }
