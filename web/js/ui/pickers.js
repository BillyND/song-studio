// Prompt snippet chips and the genre / sung-language picker.
import { html, t, useStore } from "../core.js";
import { Icon } from "../icons.js";
import { Field } from "./controls.js";

// PresetChips offers the saved prompt snippets (Cài đặt → Prompt mở rộng) as one-click inserts.
export function PresetChips({ value, onChange, genre }) {
  const own = useStore((s) => s.boot?.config?.presets || []);
  const genres = useStore((s) => s.boot?.genres || []);
  // the song's genre suggestions first, then the user's own snippets
  const presets = [...(genres.find((g) => g.id === genre)?.presets || []), ...own];
  if (!presets.length) return null;
  const add = (t) => {
    if ((value || "").includes(t)) return;
    onChange(((value || "").trim() ? value.trim() + "\n" : "") + t);
  };
  return html`<div class="preset-row">${presets.map((p) => html`
    <button type="button" class=${"preset" + ((value || "").includes(p.text) ? " on" : "")} title=${p.text} onClick=${() => add(p.text)}>
      <${Icon} name=${(value || "").includes(p.text) ? "check" : "plus"} size=${12} />${p.label}</button>`)}</div>`;
}

// GenrePicker chooses a song's genre and sung language; the sung language is independent of the UI language.
export function GenrePicker({ genre, language, onChange }) {
  const genres = useStore((s) => s.boot?.genres || []);
  const languages = useStore((s) => s.boot?.languages || []);
  return html`<div class="form-row">
    <div class="grow"><${Field} label=${t("Thể loại")} hint=${t("Chọn để agent phối và lên cảnh đúng chất; để trống thì suy ra từ bài mẫu.")}>
      <select class="select" value=${genre || ""} onChange=${(e) => onChange({ genre: e.target.value })} aria-label=${t("Thể loại")}>
        <option value="">${t("Chưa chọn")}</option>${genres.map((g) => html`<option value=${g.id}>${g.label}</option>`)}</select></${Field}></div>
    <div class="grow"><${Field} label=${t("Ngôn ngữ hát")} hint=${t("Dùng khi canh lời theo giọng hát.")}>
      <select class="select" value=${language || "vi"} onChange=${(e) => onChange({ language: e.target.value })} aria-label=${t("Ngôn ngữ hát")}>
        ${languages.map((l) => html`<option value=${l.id}>${l.label}</option>`)}</select></${Field}></div>
  </div>`;
}
