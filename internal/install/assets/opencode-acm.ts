// acm — OpenCode plugin.
//
// Auto-installed by `acm init --global opencode` into
// ~/.config/opencode/plugin/acm.ts, where OpenCode auto-loads it (Bun runs
// TypeScript natively; no build or npm step). It is intentionally thin: the
// lossless store, summary DAG, compaction, and retrieval all live in the `acm`
// binary on PATH. This plugin captures messages and tool calls into acm's
// lossless store. Drill-down (acm expand/grep/describe) is documented for the
// model in the AGENTS.md block acm init also writes.
//
// Loader constraint (OpenCode 1.17.1): export ONLY the default plugin function;
// OpenCode rejects any non-function module export.
//
// Per-session routing (works in BOTH TUI and web/server mode): the plugin learns
// each session's project directory from `session.*` events
// (event.properties.info.directory) and runs `acm` with that directory as cwd,
// so capture lands in the correct per-project .acm even when one server process
// serves many projects from a non-project cwd. Messages that arrive before their
// session's directory is known are queued and flushed once it is — so nothing is
// ever written to the wrong database. On session open the project's .acm database
// is initialized immediately (open + migrate) so it exists from the first turn.
//
// Capture detail: OpenCode carries message TEXT in `message.part.updated` events
// (part.type === "text"), not on `message.updated`'s info object, and tool calls
// in `tool` parts. We buffer text parts per message and ingest the joined text
// once final (user messages immediately, assistant messages when
// info.time.completed is set), and ingest each completed/errored tool part as a
// tool-role message. Ingestion is idempotent (keyed on message/call id). Capture
// is best-effort: it must never throw into the agent loop.
import { spawnSync } from "node:child_process"

// sessionID -> project directory (learned from session.* events).
const dirBySession = new Map<string, string>()
// sessionID -> queued ingest payloads awaiting that session's directory.
const pendingBySession = new Map<string, Record<string, unknown>[]>()
// project directories whose .acm database has already been initialized.
const initializedDirs = new Set<string>()

// Per-message text buffers.
const partsByMessage = new Map<string, Map<string, string>>() // messageID -> partID -> text
const roleByMessage = new Map<string, string>() // messageID -> role
const sessionByMessage = new Map<string, string>() // messageID -> sessionID
const ingestedMsgs = new Set<string>() // messageIDs already stored
const ingestedTools = new Set<string>() // tool callIDs already stored

// Dedup sets and per-message maps would grow forever in a long-lived server
// process. acm's ingest is idempotent (keyed on external_id), so clearing a
// full set only risks a harmless re-ingest attempt, never a duplicate row.
const MAX_TRACKED = 20_000
function bound(collection: Set<string> | Map<string, unknown>): void {
  if (collection.size > MAX_TRACKED) collection.clear()
}

function runIngest(dir: string, sessionID: string, message: Record<string, unknown>): void {
  try {
    spawnSync("acm", ["ingest"], {
      input: JSON.stringify({ agent: "opencode", session_id: sessionID, messages: [message] }),
      encoding: "utf8",
      cwd: dir,
    })
  } catch {
    /* best-effort: never break the agent */
  }
}

// enqueue routes a message to its session's database, or queues it until the
// session's directory is known (never writes to the wrong project's .acm).
function enqueue(sessionID: string, message: Record<string, unknown>): void {
  if (!sessionID || !message?.content) return
  const dir = dirBySession.get(sessionID)
  if (dir) {
    runIngest(dir, sessionID, message)
    return
  }
  const q = pendingBySession.get(sessionID) ?? []
  q.push(message)
  pendingBySession.set(sessionID, q)
}

function learnDirectory(sessionID: string, dir: string): void {
  if (!sessionID || !dir) return
  dirBySession.set(sessionID, dir)
  // Initialize the project's .acm database on session open (open + migrate), so
  // it exists from the first turn even before anything is captured — once per
  // directory. `acm doctor` is the open-and-migrate command.
  if (!initializedDirs.has(dir)) {
    initializedDirs.add(dir)
    try {
      spawnSync("acm", ["doctor"], { cwd: dir, encoding: "utf8" })
    } catch {
      /* best-effort */
    }
  }
  const queued = pendingBySession.get(sessionID)
  if (queued?.length) {
    for (const message of queued) runIngest(dir, sessionID, message)
    pendingBySession.delete(sessionID)
  }
}

function forgetSession(sessionID: string): void {
  if (!sessionID) return
  dirBySession.delete(sessionID)
  pendingBySession.delete(sessionID)
}

function rememberPart(part: any): void {
  if (!part || part.type !== "text" || typeof part.text !== "string" || !part.messageID) return
  if (ingestedMsgs.has(part.messageID)) return // already flushed; don't re-buffer
  const buf = partsByMessage.get(part.messageID) ?? new Map<string, string>()
  // Streaming text parts re-fire for the same id with growing text; latest wins.
  buf.set(part.id ?? String(buf.size), part.text)
  bound(partsByMessage)
  partsByMessage.set(part.messageID, buf)
  if (typeof part.sessionID === "string") sessionByMessage.set(part.messageID, part.sessionID)
}

function joinedText(messageID: string): string {
  return Array.from(partsByMessage.get(messageID)?.values() ?? []).filter(Boolean).join("\n")
}

function flushMessage(messageID: string): void {
  if (!messageID || ingestedMsgs.has(messageID)) return
  const role = roleByMessage.get(messageID)
  const sessionID = sessionByMessage.get(messageID)
  const text = joinedText(messageID)
  if (!role || !sessionID || !text) return
  enqueue(sessionID, { role, content: text, external_id: messageID })
  bound(ingestedMsgs)
  ingestedMsgs.add(messageID)
  partsByMessage.delete(messageID)
  roleByMessage.delete(messageID)
  sessionByMessage.delete(messageID)
}

function safeJSON(value: unknown): string {
  try {
    return JSON.stringify(value ?? {})
  } catch {
    return ""
  }
}

// captureToolPart ingests a completed/errored tool call as a tool-role message,
// once, keyed on its callID.
function captureToolPart(part: any): void {
  if (!part || part.type !== "tool" || typeof part.sessionID !== "string") return
  const state = part.state
  const status = state?.status
  if (status !== "completed" && status !== "error") return
  const id = part.callID ?? part.id
  if (!id || ingestedTools.has(id)) return
  const name = typeof part.tool === "string" ? part.tool : "tool"
  const output = status === "error" ? state.error ?? "" : typeof state.output === "string" ? state.output : safeJSON(state.output)
  const content = `[tool ${name}]\ninput: ${safeJSON(state.input)}\noutput: ${output}`
  enqueue(part.sessionID, { role: "tool", content, external_id: id, tool_name: name })
  bound(ingestedTools)
  ingestedTools.add(id)
}

export default async function acmPlugin(_ctx: any) {
  return {
    event: async ({ event }: any) => {
      try {
        const type = event?.type
        const props = event?.properties
        if (typeof type !== "string") return

        if (type.startsWith("session.")) {
          if (type === "session.deleted") {
            forgetSession(props?.info?.id ?? props?.sessionID)
            return
          }
          const info = props?.info
          if (info?.id && typeof info.directory === "string") learnDirectory(info.id, info.directory)
          return
        }

        if (type === "message.part.updated") {
          const part = props?.part
          rememberPart(part)
          captureToolPart(part)
          // User text is final on arrival; flush once we know it's a user message.
          if (part?.messageID && roleByMessage.get(part.messageID) === "user") flushMessage(part.messageID)
          return
        }

        if (type === "message.updated") {
          const info = props?.info
          if (!info?.id || !info?.role) return
          bound(roleByMessage)
          bound(sessionByMessage)
          roleByMessage.set(info.id, info.role)
          if (typeof info.sessionID === "string") sessionByMessage.set(info.id, info.sessionID)
          if (info.role === "user") {
            flushMessage(info.id)
          } else if (info.role === "assistant" && info.time?.completed) {
            flushMessage(info.id)
          }
        }
      } catch {
        /* best-effort */
      }
    },
  }
}
