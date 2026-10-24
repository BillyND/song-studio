// App shell: sidebar navigation, system meter, routing, run drawer and toasts.
import { html, render, useEffect, useErrorBoundary, api, store, useStore, useRoute, theme, toastError } from "./core.js";
import { Icon } from "./icons.js";
import { Toasts, Btn } from "./ui.js";
import { Drawer } from "./console.js";
import { ProjectsPage } from "./projects.js";
import { ProjectPage } from "./project.js";
import { RunsPage, RunPage } from "./runs.js";
import { SkillPage } from "./skill.js";
import { SettingsPage } from "./settings.js";

theme.set(new URLSearchParams(location.search).get("theme") || theme.get());

function Sidebar({ route }) {
  const system = useStore((s) => s.system);
  const boot = useStore((s) => s.boot);
  const section = route[0] || "";
  const nav = [
    { href: "#/", icon: "music", label: "Bài hát", on: section === "" || section === "p" },
    { href: "#/runs", icon: "terminal", label: "Lượt chạy", on: section === "runs", badge: system && (system.running + system.queued) || 0 },
    { href: "#/skill", icon: "code", label: "Skill", on: section === "skill" },
    { href: "#/settings", icon: "settings", label: "Cài đặt", on: section === "settings" },
  ];
  const load = system?.load?.[0] ?? 0;
  const cpus = system?.cpus || 8;
  const ratio = Math.min(1, load / cpus);
  return html`<nav class="sidebar">
    <div class="brand"><div class="brand-mark"><${Icon} name="lotus" size=${20} stroke=${2.2} /></div>
      <div class="brand-text"><div class="brand-name">Quê Hương Studio</div><div class="brand-sub">Lời → Suno → Video</div></div></div>
    ${nav.map((n) => html`<a class=${"nav-item" + (n.on ? " active" : "")} href=${n.href} title=${n.label}><${Icon} name=${n.icon} size=${17} /><span class="nav-label">${n.label}</span>
      ${n.badge ? html`<span class="nav-badge">${n.badge}</span>` : null}</a>`)}
    <div class="sidebar-foot">
      <div class="meter" title="Tải trung bình 1 phút so với số nhân CPU">
        <div class="meter-row"><span>Tải máy</span><b>${load.toFixed(1)} / ${cpus}</b></div>
        <div class="meter-bar"><i class=${ratio > 0.9 ? "high" : ratio > 0.6 ? "mid" : ""} style=${{ width: Math.max(4, ratio * 100) + "%" }}></i></div>
        <div class="meter-row"><span>Agent đang mở</span><b>${system ? Object.values(system.agentProcesses || {}).reduce((a, b) => a + b, 0) : "—"}</b></div>
      </div>
      <div class="chips">${(boot?.agents || []).map((a) => html`<span class=${"chip" + (a.available ? " on" : "")} title=${a.version || "không thấy"}><span class="dot"></span>${a.name}</span>`)}</div>
      <div class="row" style="justify-content:space-between">
        <span class="muted small">v${boot?.version}</span>
        <span class="row" style="gap:2px"><${ThemeToggle} /><${Btn} kind="ghost" size="sm" icon="stop" title="Tắt studio" onClick=${quit} /></span>
      </div>
    </div>
  </nav>`;
}

async function quit() {
  const s = store.get().system;
  const busy = s && s.running + s.queued;
  if (!confirm(busy ? `Đang có ${busy} việc chạy. Tắt studio sẽ dừng chúng. Tắt?` : "Tắt Quê Hương Studio?")) return;
  try { await api("/api/shutdown", { method: "POST" }); } catch { /* the server is already going away */ }
  document.body.innerHTML = '<div style="height:100vh;display:grid;place-items:center;font:15px system-ui;color:#999">Studio đã tắt. Mở lại từ ứng dụng Quê Hương Studio.</div>';
}

function ThemeToggle() {
  const cur = useStore((s) => s.theme);
  return html`<${Btn} kind="ghost" size="sm" icon=${cur === "dark" ? "sun" : "moon"} title="Đổi giao diện sáng/tối"
    onClick=${() => theme.set(cur === "dark" ? "light" : "dark")} />`;
}

const pageBuild = document.querySelector('meta[name="studio-build"]')?.content;

function UpdateBanner() {
  const build = useStore((s) => s.system?.build);
  if (!build || !pageBuild || build === pageBuild) return null;
  return html`<div class="update-banner"><${Icon} name="refresh" />Studio vừa được cập nhật. Tải lại trang để dùng bản mới.
    <${Btn} size="sm" kind="primary" onClick=${() => location.reload()}>Tải lại</${Btn}></div>`;
}

// Boundary keeps one broken view from taking the whole app down.
function Boundary({ children, resetKey }) {
  const [err, reset] = useErrorBoundary((e) => console.error(e));
  useEffect(() => { if (err) reset(); }, [resetKey]);
  if (err) return html`<div class="page"><div class="warn-box">Trang này gặp lỗi: ${String(err.message || err)}.
    <a style="text-decoration:underline;cursor:pointer;margin-left:8px" onClick=${reset}>Thử lại</a></div></div>`;
  return children;
}

function App() {
  const route = useRoute();
  const consoleRun = useStore((s) => s.consoleRun);
  const boot = useStore((s) => s.boot);
  useEffect(() => {
    const esc = (e) => e.key === "Escape" && !document.querySelector(".modal-back, .lightbox") && store.set({ consoleRun: null });
    addEventListener("keydown", esc);
    return () => removeEventListener("keydown", esc);
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
  return html`<div class=${"shell" + (consoleRun ? " with-drawer" : "")}>
    <${Sidebar} route=${route} />
    <main class="main">${boot && !boot.skillExists ? html`<div class="page"><div class="warn-box">Không thấy skill ở ${boot.config.skillDir}. Sửa đường dẫn trong Cài đặt.</div></div>` : null}<${Boundary} resetKey=${route.join("/")}>${page}</${Boundary}></main>
    <${Drawer} />
    <${Toasts} />
    <${UpdateBanner} />
  </div>`;
}

render(html`<${App} />`, document.getElementById("app"));
