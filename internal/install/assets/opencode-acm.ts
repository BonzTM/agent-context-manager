// acm — OpenCode plugin.
//
// Auto-installed by `acm init --global opencode` into
// ~/.config/opencode/plugin/acm.ts, where OpenCode auto-loads it (Bun runs
// TypeScript natively; no build or npm step). It is intentionally thin: the
// lossless store, summary DAG, compaction, and retrieval all live in the `acm`
// binary on PATH. This plugin captures messages and tool calls, then asks the
// binary for the active-window/recall plan used by OpenCode's transform hooks.
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

type ContextPlan = {
  version: number
  fresh_tail_messages: number
  fresh_message_ids: string[]
  summary_refs: string[]
  archive_placeholder?: string
  summary_text?: string
  recall_text?: string
  resume_note?: string
}

const ACM_ARCHIVE_PREFIX = "[Archived by acm:"
const ACM_RECALL_PREFIX = "<acm-recall>"
const MAX_CONTEXT_BYTES = 1 << 20
const MAX_TRANSFORM_MESSAGES = 5_000
const MAX_PARTS_PER_MESSAGE = 1_000
const MAX_PENDING_PER_SESSION = 1_000
const COMMAND_TIMEOUT_MS = 10_000
const lastPlanBySession = new Map<string, ContextPlan>()

// Dedup sets and per-message maps would grow forever in a long-lived server
// process. acm's ingest is idempotent (keyed on external_id), so clearing a
// full set only risks a harmless re-ingest attempt, never a duplicate row.
const MAX_TRACKED = 20_000
function bound<T>(collection: Set<T> | Map<string, T>): void {
  if (collection.size > MAX_TRACKED) collection.clear()
}

function syntheticText(text: unknown): boolean {
  if (typeof text !== "string") return false
  const trimmed = text.trimStart()
  return trimmed.startsWith(ACM_ARCHIVE_PREFIX) || trimmed.startsWith(ACM_RECALL_PREFIX)
}

function syntheticPart(part: any): boolean {
  return part?.metadata?.acmContext || (part?.synthetic === true && syntheticText(part?.text))
}

function runIngest(dir: string, sessionID: string, message: Record<string, unknown>): void {
  try {
    spawnSync("acm", ["ingest"], {
      input: JSON.stringify({ agent: "opencode", session_id: sessionID, messages: [message] }),
      encoding: "utf8",
      cwd: dir,
      timeout: COMMAND_TIMEOUT_MS,
      maxBuffer: MAX_CONTEXT_BYTES,
    })
  } catch {
    /* best-effort: never break the agent */
  }
}

// enqueue routes a message to its session's database, or queues it until the
// session's directory is known (never writes to the wrong project's .acm).
function enqueue(sessionID: string, message: Record<string, unknown>): void {
  if (!sessionID || !message?.content || syntheticText(message.content)) return
  const dir = dirBySession.get(sessionID)
  if (dir) {
    runIngest(dir, sessionID, message)
    return
  }
  const q = pendingBySession.get(sessionID) ?? []
  if (q.length >= MAX_PENDING_PER_SESSION) return
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
      spawnSync("acm", ["doctor"], {
        cwd: dir,
        encoding: "utf8",
        timeout: COMMAND_TIMEOUT_MS,
        maxBuffer: MAX_CONTEXT_BYTES,
      })
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
  lastPlanBySession.delete(sessionID)
}

function rememberPart(part: any): void {
  if (!part || part.type !== "text" || typeof part.text !== "string" || !part.messageID || syntheticPart(part)) return
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

function validPlan(value: any): value is ContextPlan {
  const boundedText = (text: unknown) => text === undefined || (typeof text === "string" && text.length <= MAX_CONTEXT_BYTES)
  return value?.version === 1 &&
    Number.isInteger(value.fresh_tail_messages) && value.fresh_tail_messages >= 0 &&
    Array.isArray(value.fresh_message_ids) && value.fresh_message_ids.length <= MAX_TRANSFORM_MESSAGES &&
    value.fresh_message_ids.every((id: unknown) => typeof id === "string") &&
    Array.isArray(value.summary_refs) && value.summary_refs.length <= 128 &&
    value.summary_refs.every((id: unknown) => typeof id === "string") &&
    boundedText(value.archive_placeholder) && boundedText(value.summary_text) &&
    boundedText(value.recall_text) && boundedText(value.resume_note)
}

function loadContextPlan(dir: string, sessionID: string, prompt: string): ContextPlan | undefined {
  try {
    const result = spawnSync("acm", ["opencode-context"], {
      input: JSON.stringify({ session_id: sessionID, prompt }),
      encoding: "utf8",
      cwd: dir,
      timeout: COMMAND_TIMEOUT_MS,
      maxBuffer: MAX_CONTEXT_BYTES,
    })
    if (result.error || result.status !== 0 || typeof result.stdout !== "string") return undefined
    const plan = JSON.parse(result.stdout)
    return validPlan(plan) ? plan : undefined
  } catch {
    return undefined
  }
}

function latestUser(messages: any[]): any | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index]?.info?.role === "user") return messages[index]
  }
  return undefined
}

function userPrompt(message: any): string {
  if (!Array.isArray(message?.parts) || message.parts.length > MAX_PARTS_PER_MESSAGE) return ""
  return message.parts
    .filter((part: any) => part?.type === "text" && typeof part.text === "string" && !syntheticPart(part))
    .map((part: any) => part.text)
    .join("\n")
}

function compactMessage(message: any, placeholder: string): boolean {
  if (!Array.isArray(message?.parts) || message.parts.length > MAX_PARTS_PER_MESSAGE) return false
  for (const part of message.parts) {
    if (syntheticPart(part)) continue
    if (part?.type === "text" || part?.type === "reasoning") part.text = placeholder
    if (part?.type === "tool" && part.state?.status === "completed") {
      part.state.output = placeholder
      part.state.attachments = undefined
    }
    if (part?.type === "tool" && part.state?.status === "error") part.state.error = placeholder
    if (part?.type === "file" && part.source?.text) {
      part.source.text.value = placeholder
      part.source.text.start = 0
      part.source.text.end = placeholder.length
    }
    if (part?.type === "snapshot") part.snapshot = placeholder
    if (part?.type === "agent" && part.source) {
      part.source.value = placeholder
      part.source.start = 0
      part.source.end = placeholder.length
    }
    if (part?.type === "patch" && Array.isArray(part.files)) part.files = part.files.slice(0, 8)
    if (part?.type === "subtask") {
      part.prompt = placeholder
      part.description = placeholder
    }
  }
  return true
}

function syntheticContextPart(anchor: any, kind: string, text: string): Record<string, unknown> {
  return {
    id: `acm-${kind}-${anchor.info.id}`,
    sessionID: anchor.info.sessionID,
    messageID: anchor.info.id,
    type: "text",
    text,
    synthetic: true,
    metadata: { acmContext: kind },
  }
}

function transformMessages(messages: any[]): void {
  if (!Array.isArray(messages) || messages.length === 0 || messages.length > MAX_TRANSFORM_MESSAGES) return
  const anchor = latestUser(messages)
  const sessionID = anchor?.info?.sessionID
  const dir = typeof sessionID === "string" ? dirBySession.get(sessionID) : undefined
  if (!anchor || !sessionID || !dir) return
  const plan = loadContextPlan(dir, sessionID, userPrompt(anchor))
  if (!plan) return
  bound(lastPlanBySession)
  lastPlanBySession.set(sessionID, plan)
  const fresh = new Set(plan.fresh_message_ids)
  const anchorIndex = messages.indexOf(anchor)
  const recentStart = Math.min(anchorIndex, Math.max(0, messages.length - plan.fresh_tail_messages))
  let archived = 0
  if (plan.archive_placeholder) {
    for (let index = 0; index < recentStart; index += 1) {
      const message = messages[index]
      if (message === anchor || fresh.has(message?.info?.id)) continue
      if (compactMessage(message, plan.archive_placeholder)) archived += 1
    }
  }
  anchor.parts = (anchor.parts ?? []).filter((part: any) => !syntheticPart(part))
  if (plan.recall_text) anchor.parts.push(syntheticContextPart(anchor, "retrieved-context", plan.recall_text))
  if (plan.summary_text && archived > 0) anchor.parts.push(syntheticContextPart(anchor, "archive-summary", plan.summary_text))
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

    "experimental.chat.messages.transform": async (_input: any, output: any) => {
      try {
        transformMessages(output?.messages)
      } catch {
        /* best-effort */
      }
    },

    "experimental.chat.system.transform": async (_input: any, output: any) => {
      const hint = "[Archived by acm: OpenCode active context may contain summary and recall parts. Use acm expand <sum-id>, acm describe <msg-id>, or acm grep <pattern> for deeper recovery.]"
      if (Array.isArray(output?.system) && !output.system.includes(hint)) output.system.push(hint)
    },

    "experimental.session.compacting": async (input: any, output: any) => {
      try {
        const sessionID = input?.sessionID
        const dir = typeof sessionID === "string" ? dirBySession.get(sessionID) : undefined
        const plan = lastPlanBySession.get(sessionID) ?? (dir ? loadContextPlan(dir, sessionID, "") : undefined)
        if (!plan?.resume_note || !Array.isArray(output?.context)) return
        if (!output.context.some((entry: unknown) => typeof entry === "string" && entry.startsWith(ACM_ARCHIVE_PREFIX))) {
          output.context.push(plan.resume_note)
        }
      } catch {
        /* best-effort */
      }
    },
  }
}
