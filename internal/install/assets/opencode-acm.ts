// acm — OpenCode plugin.
//
// Auto-installed by `acm init --global opencode` into
// ~/.config/opencode/plugin/acm.ts, where OpenCode auto-loads it (Bun runs
// TypeScript natively; no build or npm step). It is intentionally thin: the
// lossless store, summary DAG, compaction, and retrieval all live in the `acm`
// binary on PATH. This plugin only captures messages into acm's lossless store.
// Drill-down (acm expand/grep/describe) is documented for the model in the
// AGENTS.md block acm init also writes.
//
// Works in BOTH OpenCode modes:
//   - TUI (one project per process): the plugin context's directory is the
//     project, used as a fallback.
//   - Web/server (one process, many projects): the process cwd is NOT the
//     project, so we learn each session's directory from `session.updated`
//     events (event.properties.info.directory) and run `acm` with that
//     session's directory as cwd — routing each session to the correct
//     per-project .acm database.
//
// Loader constraint (OpenCode 1.17.1): export ONLY the default plugin function;
// OpenCode rejects any non-function module export.
//
// Capture detail: OpenCode carries message TEXT in `message.part.updated` events
// (part.type === "text"), not on `message.updated`'s info object. We buffer text
// parts per message id and ingest a message's joined text once final — user
// messages as soon as their text is known, assistant messages only when
// info.time.completed is set. Ingestion is idempotent (keyed on message id).
// Capture is best-effort: it must never throw into the agent loop.
import { spawnSync } from "node:child_process"

// Per-session project directory, learned from session.updated events. This is
// what makes web/server mode (many projects, one process) route correctly.
const dirBySession = new Map<string, string>()
// Fallback directory from the plugin context (correct for single-project TUI).
let fallbackDir: string | undefined

function acm(args: string[], input: string, sessionID?: string): void {
  try {
    const cwd = (sessionID ? dirBySession.get(sessionID) : undefined) ?? fallbackDir ?? undefined
    spawnSync("acm", args, { input, encoding: "utf8", cwd })
  } catch {
    /* best-effort: never break the agent */
  }
}

// Per-message buffers, learned incrementally from the event stream.
const partsByMessage = new Map<string, Map<string, string>>() // messageID -> partID -> text
const roleByMessage = new Map<string, string>() // messageID -> role
const sessionByMessage = new Map<string, string>() // messageID -> sessionID
const ingested = new Set<string>() // messageIDs already stored

function rememberPart(part: any): void {
  if (!part || part.type !== "text" || typeof part.text !== "string" || !part.messageID) return
  const buf = partsByMessage.get(part.messageID) ?? new Map<string, string>()
  // Streaming text parts re-fire for the same id with growing text; the latest
  // value wins. Map iteration is insertion order, preserving part order.
  buf.set(part.id ?? String(buf.size), part.text)
  partsByMessage.set(part.messageID, buf)
  if (typeof part.sessionID === "string") sessionByMessage.set(part.messageID, part.sessionID)
}

function joinedText(messageID: string): string {
  return Array.from(partsByMessage.get(messageID)?.values() ?? []).filter(Boolean).join("\n")
}

function flush(messageID: string): void {
  if (!messageID || ingested.has(messageID)) return
  const role = roleByMessage.get(messageID)
  const sessionID = sessionByMessage.get(messageID)
  const text = joinedText(messageID)
  if (!role || !sessionID || !text) return
  acm(
    ["ingest"],
    JSON.stringify({
      agent: "opencode",
      session_id: sessionID,
      messages: [{ role, content: text, external_id: messageID }],
    }),
    sessionID,
  )
  ingested.add(messageID)
  partsByMessage.delete(messageID)
}

export default async function acmPlugin(ctx: any) {
  fallbackDir = ctx?.directory ?? ctx?.worktree ?? undefined
  return {
    event: async ({ event }: any) => {
      try {
        const type = event?.type
        const props = event?.properties
        if (type === "session.updated") {
          const info = props?.info
          if (info?.id && typeof info.directory === "string") dirBySession.set(info.id, info.directory)
          return
        }
        if (type === "message.part.updated") {
          const part = props?.part
          rememberPart(part)
          // If we already know this is a user message, its text is final on
          // arrival — flush now (handles part-after-message ordering).
          if (part?.messageID && roleByMessage.get(part.messageID) === "user") flush(part.messageID)
          return
        }
        if (type === "message.updated") {
          const info = props?.info
          if (!info?.id || !info?.role) return
          roleByMessage.set(info.id, info.role)
          if (typeof info.sessionID === "string") sessionByMessage.set(info.id, info.sessionID)
          if (info.role === "user") {
            flush(info.id) // user text is final; flush whatever has arrived
          } else if (info.role === "assistant" && info.time?.completed) {
            flush(info.id) // assistant done: store the full streamed reply
          }
        }
      } catch {
        /* best-effort */
      }
    },
  }
}
