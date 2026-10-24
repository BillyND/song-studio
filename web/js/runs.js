// Run history and the full-page run view.
import { html, useState, useEffect, api, openRun, go, ago, useStore, runDuration } from "./core.js";
import { Icon } from "./icons.js";
import { Seg, StatusPill, AgentIcon, Empty, Btn } from "./ui.js";
import { RunView } from "./console.js";

export function RunsPage() {
  const [runs, setRuns] = useState(null);
  const [filter, setFilter] = useState("all");
  const runsVersion = useStore((s) => s.runsVersion);
  const load = () => api("/api/runs?limit=200").then(setRuns).catch(() => {});
  useEffect(() => { load(); }, [runsVersion]);
  useEffect(() => {
    if (!runs?.some((r) => r.status === "running" || r.status === "queued")) return;
    const t = setInterval(load, 2000);
    return () => clearInterval(t);
  }, [runs]);
  const shown = (runs || []).filter((r) => filter === "all" || (filter === "active" ? ["running", "queued"].includes(r.status) : filter === "agent" ? r.kind === "agent" : r.kind === "script"));
  return html`<div class="page">
    <div class="page-head"><div><div class="eyebrow">Lịch sử</div><h1>Lượt chạy</h1>
      <div class="muted" style="margin-top:6px">Mọi script và agent đã chạy, kể cả khi studio đã tắt rồi mở lại.</div></div>
      <${Seg} value=${filter} onChange=${setFilter} options=${[{ value: "all", label: "Tất cả" }, { value: "active", label: "Đang chạy" }, { value: "agent", label: "Agent" }, { value: "script", label: "Script" }]} /></div>
    ${runs && shown.length === 0 ? html`<${Empty} icon="terminal" title="Chưa có lượt chạy">Các bước trong mỗi bài sẽ xuất hiện ở đây.</${Empty}>` : html`
    <div class="card"><table class="table runs-table"><thead><tr><th></th><th>Việc</th><th>Bài</th><th>Trạng thái</th><th>Thời gian</th><th>Chi phí</th><th>Lúc</th></tr></thead><tbody>
      ${shown.map((r) => html`<tr onClick=${() => openRun(r.id)} onDblClick=${() => go("/runs/" + r.id)}>
        <td style="width:34px"><${AgentIcon} agent=${r.kind === "agent" ? r.agent : "script"} /></td>
        <td><div style="font-weight:600">${r.title}</div><div class="muted small mono">${r.id}</div></td>
        <td class="small">${r.project ? html`<a href=${"#/p/" + encodeURIComponent(r.project)} onClick=${(e) => e.stopPropagation()}>${r.project}</a>` : r.step === "skill" ? "skill" : "—"}</td>
        <td><${StatusPill} status=${r.status} /></td><td class="num">${runDuration(r)}</td>
        <td class="num">${r.usage?.costUsd ? "$" + r.usage.costUsd.toFixed(3) : ""}</td><td class="small muted">${ago(r.created)}</td></tr>`)}
    </tbody></table></div>`}
  </div>`;
}

export function RunPage({ id }) {
  return html`<div class="page" style="height:100vh;display:flex;flex-direction:column;padding-bottom:20px">
    <div class="row" style="margin-bottom:14px"><${Btn} kind="ghost" size="sm" icon="back" onClick=${() => history.back()}>Quay lại</${Btn}></div>
    <div class="card grow" style="min-height:0;overflow:hidden;display:flex;flex-direction:column"><${RunView} id=${id} /></div>
  </div>`;
}
