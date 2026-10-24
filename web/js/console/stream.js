// A run's live event stream (SSE with resume) and the grouping that pairs tool calls with results.
import { api, bumpRuns, isActive, useEffect, useState } from "../core.js";

export function useRunStream(id) {
  const [meta, setMeta] = useState(null);
  const [events, setEvents] = useState([]);
  useEffect(() => {
    if (!id) return;
    setMeta(null);
    setEvents([]);
    let last = 0;
    let es = null;
    let closed = false;
    let wasActive = false;
    const connect = () => {
      es = new EventSource(`/api/runs/${encodeURIComponent(id)}/events?after=${last}`);
      es.onmessage = (m) => {
        const e = JSON.parse(m.data);
        if (e.seq <= last) return;
        if (e.seq !== last + 1) { es.close(); setTimeout(() => !closed && connect(), 100); return; }
        last = e.seq;
        setEvents((list) => [...list, e]);
      };
      es.addEventListener("meta", (m) => {
        const next = JSON.parse(m.data);
        if (isActive(next.status)) wasActive = true;
        setMeta(next);
      });
      es.addEventListener("end", (m) => {
        setMeta(JSON.parse(m.data));
        closed = true;
        es.close();
        if (wasActive) bumpRuns();
      });
      es.onerror = () => {
        if (closed) return;
        es.close();
        setTimeout(() => !closed && connect(), 1500); // the server may have restarted; resume after the last seq
      };
    };
    connect();
    return () => { closed = true; es && es.close(); };
  }, [id]);
  return { meta, events };
}

// Pair tool calls with their results and merge consecutive log lines into one block.
export function groupEvents(events) {
  const out = [];
  const byRef = new Map();
  let lastText = "";
  for (const e of events) {
    if (e.kind === "text") lastText = e.body;
    if (e.kind === "result" && e.body && e.body.trim() === lastText.trim()) {
      out.push({ ...e, body: "" }); // the CLI repeats its final reply in the result; show it once
      continue;
    }
    if (e.kind === "tool_result" && e.ref && byRef.has(e.ref)) {
      byRef.get(e.ref).result = e;
      continue;
    }
    if (e.kind === "log" || e.kind === "stderr") {
      const prev = out[out.length - 1];
      if (prev && prev.kind === "logs") { prev.lines.push(e); continue; }
      out.push({ kind: "logs", seq: e.seq, lines: [e] });
      continue;
    }
    const item = { ...e };
    if (e.kind === "tool_call" && e.ref) byRef.set(e.ref, item);
    out.push(item);
  }
  return out;
}
