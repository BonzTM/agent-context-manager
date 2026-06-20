// acm — OpenCode plugin.
//
// Auto-installed by `acm init --global opencode` into
// ~/.config/opencode/plugin/acm.ts, where OpenCode auto-loads it (Bun runs
// TypeScript natively; no build or npm step). It is intentionally thin: the
// lossless store, summary DAG, compaction, and retrieval all live in the `acm`
// binary on PATH. This plugin only captures each completed message into acm's
// lossless store. Drill-down (acm expand/grep/describe) is documented for the
// model in the AGENTS.md block acm init also writes.
//
// Capture is best-effort: it must never throw into the agent loop. Field access
// is defensive so a schema change in a future OpenCode version degrades to "no
// capture" rather than breaking the agent.
import { spawnSync } from "node:child_process"

export const id = "acm"

function acm(args: string[], input?: string): void {
  try {
    spawnSync("acm", args, { input: input ?? "", encoding: "utf8" })
  } catch {
    /* best-effort: never break the agent */
  }
}

function textOf(info: any): string {
  if (!info) return ""
  if (typeof info.text === "string") return info.text
  if (Array.isArray(info.parts)) {
    return info.parts
      .map((p: any) => (typeof p?.text === "string" ? p.text : ""))
      .filter(Boolean)
      .join("\n")
  }
  return ""
}

export default async function acmPlugin(_input: any) {
  return {
    // Archive each completed message into acm's lossless store.
    event: async ({ event }: any) => {
      try {
        if (event?.type !== "message.updated") return
        const info = event.properties?.info ?? event.properties?.message
        const role = info?.role
        const sessionID = info?.sessionID ?? event.properties?.sessionID
        const text = textOf(info)
        if (!role || !sessionID || !text) return
        acm(
          ["ingest"],
          JSON.stringify({
            agent: "opencode",
            session_id: sessionID,
            messages: [{ role, content: text, external_id: info?.id ?? "" }],
          }),
        )
      } catch {
        /* best-effort */
      }
    },
  }
}
