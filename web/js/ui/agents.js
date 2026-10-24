// Agent badge and the agent/model picker used by every agent action.
import { html, t, useEffect, useState, useStore } from "../core.js";
import { Seg } from "./controls.js";

export function AgentIcon({ agent }) {
  const letter = { claude: "C", omp: "O", codex: "X", script: "›_" }[agent] || "?";
  return html`<span class=${"agent-ico " + (agent || "script")} title=${agent || "script"}>${letter}</span>`;
}

// AgentPicker lets each agent action choose its CLI and optional model.
export function AgentPicker({ value, onChange, model, onModel }) {
  const boot = useStore((s) => s.boot);
  const agents = boot?.agents || [];
  return html`<span class="agent-pick">
    <${Seg} value=${value} onChange=${onChange} options=${agents.map((a) => ({ value: a.name, label: a.available ? a.name : t("{name} (thiếu)", { name: a.name }) }))} />
    ${onModel && html`<input class="input" style="width:150px;height:34px" placeholder=${t("model (mặc định)")} value=${model} onInput=${(e) => onModel(e.target.value)} />`}
  </span>`;
}

export function useAgentChoice() {
  const def = useStore((s) => s.boot?.config?.defaultAgent || "claude");
  const [agent, setAgent] = useState(def);
  const [model, setModel] = useState("");
  useEffect(() => setAgent(def), [def]);
  return { agent, setAgent, model, setModel };
}
