// acm-opencode — the OpenCode adapter for agent-context-manager (acm).
//
// OpenCode is the one host agent whose plugin API can deterministically own the
// active context window (experimental.chat.messages.transform). This plugin is
// intentionally thin: the "brain" (lossless store, summary DAG, compaction,
// retrieval) lives in the Go `acm` binary on PATH; the plugin only:
//
//   1. captures every message into acm's lossless store (event hook), and
//   2. tells the model that acm's drill-down commands exist (system transform).
//
// Window assembly (rewriting output.messages from `acm window <session>`) is the
// advanced path and is scaffolded but left off by default — see assembleWindow.
//
// Field names below follow the OpenCode plugin SDK; access is defensive (?. and
// try/catch) so a schema drift in a future OpenCode version degrades to "no
// capture" rather than breaking the agent. acm resolves the per-project DB from
// the working directory (or $LCM_DB / $CLAUDE_PROJECT_DIR).
import { spawnSync } from "node:child_process"

function acm(args: string[], input?: string): { ok: boolean; stdout: string } {
  try {
    const res = spawnSync("acm", args, { input: input ?? "", encoding: "utf8" })
    return { ok: res.status === 0, stdout: res.stdout ?? "" }
  } catch {
    return { ok: false, stdout: "" }
  }
}

function extractText(part: any): string {
  if (!part) return ""
  if (typeof part.text === "string") return part.text
  if (Array.isArray(part.parts)) {
    return part.parts
      .map((p: any) => (typeof p?.text === "string" ? p.text : ""))
      .filter(Boolean)
      .join("\n")
  }
  return ""
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export default async function AcmPlugin(_ctx: any) {
  return {
    // Capture: archive each completed message into acm's lossless store.
    // Best-effort: capture must never throw into the agent loop.
    event: async ({ event }: any) => {
      try {
        if (event?.type !== "message.updated") return
        const info = event.properties?.info ?? event.properties?.message
        const role = info?.role
        const sessionID = info?.sessionID ?? event.properties?.sessionID
        const text = extractText(info)
        if (!role || !sessionID || !text) return
        const payload = JSON.stringify({
          agent: "opencode",
          session_id: sessionID,
          messages: [{ role, content: text, external_id: info?.id ?? "" }],
        })
        acm(["ingest"], payload)
      } catch {
        /* best-effort capture */
      }
    },

    // Inject a drill-down hint so the model knows acm's recall commands exist.
    "experimental.chat.system.transform": async (_input: any, output: any) => {
      try {
        output.system = output.system ?? []
        output.system.push(
          "This project uses acm (agent-context-manager) for long-context management. " +
            "To recover compacted or older context, run in your shell: " +
            '`acm grep "<pattern>"`, `acm expand <id>`, or `acm describe <id>`.',
        )
      } catch {
        /* best-effort */
      }
    },
  }
}

// assembleWindow is the advanced, opt-in path: deterministic active-window
// ownership. Wire it into experimental.chat.messages.transform once you are
// ready for acm to fully own what OpenCode sends the model:
//
//   "experimental.chat.messages.transform": async (_input, output) => {
//     const r = acm(["window", sessionID, "--json"])
//     if (r.ok) output.messages = toOpenCodeMessages(JSON.parse(r.stdout))
//   }
//
// It is left off by default so capture+hint works immediately without risking
// the live message array; enable it after validating the mapping for your
// OpenCode version.
