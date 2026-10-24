// Updates (inside the Mac app only): whether Sparkle checks on its own and whether it installs
// without asking. The switches take effect at once; Sparkle keeps them in its own defaults.
import { html, t, toastError, useEffect, useState } from "../core.js";
import { Btn, Field, Switch } from "../ui.js";

const bridge = () => window.webkit?.messageHandlers?.updates;

export function UpdatesCard() {
  const [s, setS] = useState(null);
  const send = (msg) => bridge().postMessage(msg).then(setS).catch(toastError);
  useEffect(() => { if (bridge()) send({ action: "get" }); }, []);
  if (!bridge() || !s) return null; // a browser tab has no updater
  return html`<div class="card card-pad stack">
    <h2>${t("Cập nhật")}</h2>
    <${Switch} on=${s.autoCheck} onChange=${(v) => send({ action: "set", autoCheck: v })}>${t("Tự kiểm tra bản mới")}</${Switch}>
    <${Switch} on=${s.autoCheck && s.autoInstall} onChange=${(v) => s.autoCheck && send({ action: "set", autoInstall: v })}>
      <span style=${s.autoCheck ? {} : { opacity: 0.5 }}>${t("Tự tải và cài bản mới")}</span></${Switch}>
    <${Field} label=${t("Phiên bản")} hint=${s.autoInstall ? t("Bản mới được tải sẵn và cài khi bạn thoát app (chờ các việc đang chạy xong).") : t("Có bản mới, app hỏi bạn trước khi tải và cài.")}>
      <div class="row"><span class="mono">${s.version}</span>
        <${Btn} size="sm" icon="refresh" disabled=${!s.canCheck} onClick=${() => send({ action: "check" })}>${t("Kiểm tra cập nhật")}</${Btn}></div></${Field}>
  </div>`;
}
