// Shared runtime: Preact bindings, API client, a tiny global store and formatting helpers.
import { h, render } from "/lib/preact.mjs";
import { useState, useEffect, useRef, useMemo, useCallback, useLayoutEffect, useErrorBoundary } from "/lib/hooks.mjs";
import htm from "/lib/htm.mjs";
import EN_SHELL from "./i18n/en-shell.js";
import EN_PROJECT from "./i18n/en-project.js";
import { localStorageGet, localStorageSet } from "./core/prefs.js";
export * from "./core/format.js";
export * from "./core/prefs.js";

export const html = htm.bind(h);
export { h, render, useState, useEffect, useRef, useMemo, useCallback, useLayoutEffect, useErrorBoundary };

const token = document.querySelector('meta[name="studio-token"]')?.content || "";

// ---------- language ----------
// UI text is written in Vietnamese and passed through t(); the English dictionaries map each
// Vietnamese string (with {name} placeholders) to its English version. The server writes the
// chosen language into the page, and changing it in Cài đặt reloads the page.
const EN = { ...EN_SHELL, ...EN_PROJECT };
export const lang = document.querySelector('meta[name="studio-lang"]')?.content === "en" ? "en" : "vi";

export function t(vi, params) {
  let s = (lang === "en" && EN[vi]) || vi;
  if (params) s = s.replace(/\{(\w+)\}/g, (m, k) => (k in params ? String(params[k]) : m));
  return s;
}

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
  chatOpen: localStorageGet("chatOpen") === "1", // the agent chat panel on the right
  chatId: localStorageGet("chatId") || null, // the conversation shown there; null starts a new one
  skillFile: "", // the file open on the Skill page, for the chat's sense of where the user is
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

// isActive: a run (or a chat's latest turn) that is still queued or running.
export const isActive = (status) => status === "queued" || status === "running";

export function openRun(id) { store.set({ consoleRun: id }); }
// openChat shows the chat panel on a conversation (or a new one when id is null). The panel and
// the run console share the right column, so the console makes way.
export function openChat(id = null) {
  store.set({ chatOpen: true, chatId: id, consoleRun: null });
  localStorageSet("chatOpen", "1");
  localStorageSet("chatId", id || "");
}
// where tells the chat agent which screen the user is on (sent with every message; the server
// only mentions it to the agent when it changed).
export function where() {
  const r = (location.hash || "#/").replace(/^#/, "").split("/").filter(Boolean).map(decodeURIComponent);
  switch (r[0]) {
    case "p": return { page: "project", project: r[1], step: r[2] || "" };
    case "skill": return { page: "skill", file: state.skillFile || "" };
    case "runs": return { page: "runs", run: r[1] || state.consoleRun || "" };
    case "settings": return { page: "settings" };
    default: return { page: "songs" };
  }
}
export function closeChat() {
  store.set({ chatOpen: false });
  localStorageSet("chatOpen", "0");
}
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
