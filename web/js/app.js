// App shell: sidebar navigation, system meter, routing, run drawer and toasts.
import { html, render, useEffect, useErrorBoundary, api, store, useStore, useRoute, theme, toastError, t, openChat, closeChat, localStorageGet } from "./core.js";
import { Icon } from "./icons.js";
import { Toasts, Btn } from "./ui.js";
import { Drawer } from "./console.js";
import { ChatPanel } from "./chat.js";
import { ProjectsPage } from "./projects.js";
import { ProjectPage } from "./project.js";
import { RunsPage, RunPage } from "./runs.js";
import { SkillPage } from "./skill.js";
import { SettingsPage } from "./settings.js";

theme.set(new URLSearchParams(location.search).get("theme") || theme.get());
const drawerW = localStorageGet("drawerW");
if (drawerW) document.documentElement.style.setProperty("--drawer", drawerW);

const toggleChat = () => (store.get().chatOpen && !store.get().consoleRun ? closeChat() : openChat(store.get().chatId));

function Sidebar({ route }) {
  const system = useStore((s) => s.system);
  const boot = useStore((s) => s.boot);
  const section = route[0] || "";
  const nav = [
    { href: "#/", icon: "music", label: t("Bài hát"), on: section === "" || section === "p" },
    { href: "#/runs", icon: "terminal", label: t("Lượt chạy"), on: section === "runs", badge: system && (system.running + system.queued) || 0 },
    { href: "#/skill", icon: "code", label: "Skill", on: section === "skill" },
    { href: "#/settings", icon: "settings", label: t("Cài đặt"), on: section === "settings" },
  ];
  const load = system?.load?.[0] ?? 0;
  const cpus = system?.cpus || 8;
  const ratio = Math.min(1, load / cpus);
  return html`<nav class="sidebar">
    <div class="brand"><div class="brand-mark"><${Icon} name="lotus" size=${20} stroke=${2.2} /></div>
      <div class="brand-text"><div class="brand-name">Song Studio</div><div class="brand-sub">${t("Lời → Suno → Video")}</div></div></div>
    ${nav.map((n) => html`<a class=${"nav-item" + (n.on ? " active" : "")} href=${n.href} title=${n.label}><${Icon} name=${n.icon} size=${17} /><span class="nav-label">${n.label}</span>
      ${n.badge ? html`<span class="nav-badge">${n.badge}</span>` : null}</a>`)}
    <div class="sidebar-foot">
      <div class="meter" title=${t("Tải trung bình 1 phút so với số nhân CPU")}>
        <div class="meter-row"><span>${t("Tải máy")}</span><b>${load.toFixed(1)} / ${cpus}</b></div>
        <div class="meter-bar"><i class=${ratio > 0.9 ? "high" : ratio > 0.6 ? "mid" : ""} style=${{ width: Math.max(4, ratio * 100) + "%" }}></i></div>
        <div class="meter-row"><span>${t("Agent đang mở")}</span><b>${system ? Object.values(system.agentProcesses || {}).reduce((a, b) => a + b, 0) : "—"}</b></div>
      </div>
      <div class="chips">${(boot?.agents || []).map((a) => html`<span class=${"chip" + (a.available ? " on" : "")} title=${a.version || t("không thấy")}><span class="dot"></span>${a.name}</span>`)}</div>
      <div class="row" style="justify-content:space-between">
        <span class="muted small">v${boot?.version}</span>
        <${ThemeToggle} />
      </div>
    </div>
  </nav>`;
}

// The agent chat lives in a floating bubble, apart from the page navigation; it hides while the
// chat panel is open.
function ChatBubble() {
  const shown = useStore((s) => s.chatOpen && !s.consoleRun);
  if (shown) return null;
  return html`<button type="button" class="chat-bubble" title=${t("Chat với agent (⌘J)")} aria-label=${t("Chat với agent (⌘J)")} onClick=${toggleChat}>
    <${Icon} name="bot" size=${22} /></button>`;
}

function ThemeToggle() {
  const cur = useStore((s) => s.theme);
  return html`<${Btn} kind="ghost" size="sm" icon=${cur === "dark" ? "sun" : "moon"} title=${t("Đổi giao diện sáng/tối")}
    onClick=${() => theme.set(cur === "dark" ? "light" : "dark")} />`;
}

const pageBuild = document.querySelector('meta[name="studio-build"]')?.content;

function UpdateBanner() {
  const build = useStore((s) => s.system?.build);
  if (!build || !pageBuild || build === pageBuild) return null;
  return html`<div class="update-banner"><${Icon} name="refresh" />${t("Studio vừa được cập nhật. Tải lại trang để dùng bản mới.")}
    <${Btn} size="sm" kind="primary" onClick=${() => location.reload()}>${t("Tải lại")}</${Btn}></div>`;
}

// Boundary keeps one broken view from taking the whole app down.
function Boundary({ children, resetKey }) {
  const [err, reset] = useErrorBoundary((e) => console.error(e));
  useEffect(() => { if (err) reset(); }, [resetKey]);
  if (err) return html`<div class="page"><div class="warn-box">${t("Trang này gặp lỗi: {error}.", { error: String(err.message || err) })}
    <a style="text-decoration:underline;cursor:pointer;margin-left:8px" onClick=${reset}>${t("Thử lại")}</a></div></div>`;
  return children;
}

function App() {
  const route = useRoute();
  const consoleRun = useStore((s) => s.consoleRun);
  const chatOpen = useStore((s) => s.chatOpen);
  const boot = useStore((s) => s.boot);
  useEffect(() => {
    const keys = (e) => {
      if (e.key === "Escape" && !document.querySelector(".modal-back, .lightbox")) store.set({ consoleRun: null });
      if ((e.metaKey || e.ctrlKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === "j") { e.preventDefault(); toggleChat(); }
    };
    addEventListener("keydown", keys);
    return () => removeEventListener("keydown", keys);
  }, []);
  useEffect(() => {
    api("/api/bootstrap").then((b) => store.set({ boot: b })).catch(toastError);
    const poll = () => api("/api/system").then((s) => store.set({ system: s })).catch(() => {});
    poll();
    const t = setInterval(poll, 5000);
    return () => clearInterval(t);
  }, []);
  let page;
  switch (route[0]) {
    case "p": page = html`<${ProjectPage} key=${route[1]} slug=${route[1]} step=${route[2]} />`; break;
    case "runs": page = route[1] ? html`<${RunPage} key=${route[1]} id=${route[1]} />` : html`<${RunsPage} />`; break;
    case "skill": page = html`<${SkillPage} />`; break;
    case "settings": page = html`<${SettingsPage} />`; break;
    default: page = html`<${ProjectsPage} />`;
  }
  return html`<div class=${"shell" + (consoleRun || chatOpen ? " with-drawer" : "")}>
    <${Sidebar} route=${route} />
    <main class="main">${boot && !boot.skillExists ? html`<div class="page"><div class="warn-box">${t("Không thấy skill ở {path}. Sửa đường dẫn trong Cài đặt.", { path: boot.config.skillDir })}</div></div>` : null}<${Boundary} resetKey=${route.join("/")}>${page}</${Boundary}></main>
    ${consoleRun ? html`<${Drawer} />` : chatOpen ? html`<${ChatPanel} />` : null}
    <${ChatBubble} />
    <${Toasts} />
    <${UpdateBanner} />
  </div>`;
}

render(html`<${App} />`, document.getElementById("app"));
