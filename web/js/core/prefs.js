// Per-viewer preferences kept in this browser: the theme and small remembered choices.
import { store } from "../core.js";

export const theme = {
  get: () => localStorageGet("theme") || "dark",
  // Switching themes restyles every element. Per-element colour transitions made that look like
  // lag (each part faded at its own pace), so they are paused for the switch. No View Transition:
  // in the app's WKWebView its callback did not run, so the theme never changed.
  set(v) {
    const root = document.documentElement;
    if (store.get().theme !== v) store.set({ theme: v }); // the icon flips now, even while the page crossfades
    if (root.dataset.theme === v) return;
    root.classList.add("theme-switching");
    root.dataset.theme = v;
    localStorageSet("theme", v);
    setTimeout(() => root.classList.remove("theme-switching"), 50);
  },
};
export function localStorageGet(k) { try { return localStorage.getItem("qhs." + k); } catch { return null; } }
export function localStorageSet(k, v) { try { localStorage.setItem("qhs." + k, v); } catch { /* private mode */ } }
