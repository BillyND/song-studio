// What a chat works on (a song, the skill, all songs) and the chip that shows it.
import { api, html, t, useEffect, useState } from "../core.js";
import { Icon } from "../icons.js";

export const RECENT_TURNS = 8;

// Song titles for context chips, fetched once per page load.
let titlesCache = null;

export function useTitles() {
  const [titles, setTitles] = useState(titlesCache || {});
  useEffect(() => {
    if (titlesCache) return;
    api("/api/projects").then((list) => {
      titlesCache = Object.fromEntries(list.map((p) => [p.slug, p.title]));
      setTitles(titlesCache);
    }).catch(() => {});
  }, []);
  return titles;
}

export function contextOf(c, titles) {
  if (c.target === "project") return { icon: "music", label: titles[c.project] || c.project };
  if (c.target === "skill") return { icon: "code", label: "Skill" };
  return { icon: "folder", label: t("Mọi bài hát") };
}

export function ContextChip({ ctx, onRemove }) {
  const titles = useTitles();
  const c = contextOf(ctx, titles);
  return html`<span class="ctx-chip" title=${t("Agent làm việc trong thư mục này")}><${Icon} name=${c.icon} size=${12} /><span class="ellipsis">${c.label}</span>
    ${onRemove && html`<button type="button" title=${t("Bỏ ngữ cảnh này")} onClick=${onRemove}><${Icon} name="x" size=${11} /></button>`}</span>`;
}
