// acm — OpenCode plugin (OpenCode 2.x plugin API).
//
// Auto-installed by `acm init --global opencode` into
// ~/.config/opencode/plugins/acm/server.ts, where OpenCode auto-loads it as a
// local server plugin (Bun runs TypeScript natively; no build or npm step). It
// is intentionally thin: the lossless store, summary DAG, compaction, and
// retrieval all live in the `acm` binary on PATH. This plugin captures messages
// and tool calls, then asks the binary for the active-window/recall plan applied
// in OpenCode's session `context` hook.
//
// Loader contract (OpenCode 2.x): the module's default export must be a plugin
// definition `{ id, setup }`. OpenCode creates one plugin instance per location
// (project directory) and passes it as `ctx.location`; the event stream spans
// every location, so each instance handles only events for its own directory
// and runs `acm` with that directory as cwd.
//
// Capture (from `ctx.event.subscribe`, best-effort):
//   user       session.inbox.enqueued (item.type "user") buffered by inboxID,
//              ingested on session.inbox.delivered, dropped on cancel.
//   assistant  session.text.ended buffered per step (assistantMessageID, ordinal),
//              ingested joined on session.step.ended / session.step.failed.
//   tool       session.tool.input.started (name) + session.tool.called (input),
//              ingested on session.tool.success / session.tool.failed.
// The session `context` hook also ingests the current user message (same id as
// its inboxID), because it can fire before the stream delivers that prompt.
// Ingestion is idempotent (keyed on external_id) and serialized in event order.
// The stream is volatile: events emitted while disconnected are not replayed.
// Capture and context shaping must never throw into the agent loop.
import { spawn } from "node:child_process"

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

type Ingest = { role: string; content: string; external_id: string; tool_name?: string }

const ACM_ARCHIVE_PREFIX = "[Archived by acm:"
const ACM_RECALL_PREFIX = "<acm-recall>"
const SYSTEM_HINT =
  "[Archived by acm: OpenCode active context may contain summary and recall parts. Use acm expand <sum-id>, acm describe <msg-id>, or acm grep <pattern> for deeper recovery.]"
const MAX_CONTEXT_BYTES = 1 << 20
const MAX_TRANSFORM_MESSAGES = 5_000
const MAX_PARTS_PER_MESSAGE = 1_000
const MAX_QUEUED_COMMANDS = 1_000
const MAX_TRACKED = 20_000
const MAX_CONSECUTIVE_RECONNECTS = 20
const RECONNECT_BASE_MS = 1_000
const RECONNECT_CAP_MS = 30_000
const COMMAND_TIMEOUT_MS = 10_000
const QUEUE_DRAIN_TIMEOUT_MS = 5_000

// Per-instance maps would grow forever in a long-lived server process. acm's
// ingest is idempotent, so clearing a full map only risks losing an in-flight
// buffer, never a duplicate row.
function bound(collection: Map<string, unknown>): void {
  if (collection.size > MAX_TRACKED) collection.clear()
}

function syntheticText(text: unknown): boolean {
  if (typeof text !== "string") return false
  const trimmed = text.trimStart()
  return trimmed.startsWith(ACM_ARCHIVE_PREFIX) || trimmed.startsWith(ACM_RECALL_PREFIX)
}

function acmPart(part: any): boolean {
  return Boolean(part?.metadata?.acmContext)
}

function safeJSON(value: unknown): string {
  try {
    return JSON.stringify(value ?? {})
  } catch {
    return ""
  }
}

function delay(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const done = () => {
      clearTimeout(timer)
      signal.removeEventListener("abort", done)
      resolve()
    }
    const timer = setTimeout(done, ms)
    signal.addEventListener("abort", done, { once: true })
  })
}

// runAcm runs `acm <args>` in dir with input on stdin. It resolves stdout on a
// zero exit, or undefined on any failure, timeout, or oversized output.
function runAcm(dir: string, args: string[], input: string): Promise<string | undefined> {
  return new Promise((resolve) => {
    let out = ""
    let settled = false
    const finish = (value: string | undefined) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      resolve(value)
    }
    let child: ReturnType<typeof spawn>
    try {
      child = spawn("acm", args, { cwd: dir, stdio: ["pipe", "pipe", "ignore"] })
    } catch {
      return finish(undefined)
    }
    const timer = setTimeout(() => (child.kill("SIGKILL"), finish(undefined)), COMMAND_TIMEOUT_MS)
    child.stdout?.setEncoding("utf8")
    child.stdout?.on("data", (chunk: string) => {
      out += chunk
      if (out.length > MAX_CONTEXT_BYTES) (child.kill("SIGKILL"), finish(undefined))
    })
    child.on("error", () => finish(undefined))
    child.on("close", (code) => finish(code === 0 ? out : undefined))
    child.stdin?.on("error", () => {})
    child.stdin?.end(input)
  })
}

// createQueue serializes acm commands so ingestion keeps event order and never
// blocks the server's event loop. It drops work beyond MAX_QUEUED_COMMANDS.
function createQueue() {
  let tail: Promise<unknown> = Promise.resolve()
  let pending = 0
  return {
    push(task: () => Promise<unknown>): void {
      if (pending >= MAX_QUEUED_COMMANDS) return
      pending += 1
      tail = tail.then(task).catch(() => undefined).finally(() => (pending -= 1))
    },
    // drain waits for queued commands, bounded by QUEUE_DRAIN_TIMEOUT_MS.
    drain(): Promise<unknown> {
      let timer: ReturnType<typeof setTimeout> | undefined
      const timeout = new Promise((resolve) => (timer = setTimeout(resolve, QUEUE_DRAIN_TIMEOUT_MS)))
      return Promise.race([tail, timeout]).finally(() => clearTimeout(timer))
    },
  }
}

type Queue = ReturnType<typeof createQueue>

function toolOutput(data: any): string {
  const parts = Array.isArray(data?.content) ? data.content : []
  const text = parts
    .map((part: any) => (part?.type === "text" ? part.text : part?.type === "file" ? `[file ${part.name ?? part.uri}]` : ""))
    .filter(Boolean)
    .join("\n")
  if (typeof data?.error?.message !== "string") return text
  return text ? `${data.error.message}\n${text}` : data.error.message
}

// createSink queues `acm` ingestion for one location. The first use runs
// `acm doctor`, which opens and migrates the project's .acm database.
function createSink(directory: string, queue: Queue) {
  let initialized = false
  return (sessionID: unknown, message: Ingest): void => {
    if (typeof sessionID !== "string" || !sessionID || !message.content || syntheticText(message.content)) return
    if (!initialized) {
      initialized = true
      queue.push(() => runAcm(directory, ["doctor"], ""))
    }
    const payload = JSON.stringify({ agent: "opencode", session_id: sessionID, messages: [message] })
    queue.push(() => runAcm(directory, ["ingest"], payload))
  }
}

type Sink = ReturnType<typeof createSink>

// createCapture returns the event handler for one location. It buffers partial
// turns and sends finished messages to the sink.
function createCapture(directory: string, ingest: Sink) {
  const prompts = new Map<string, string>() // inboxID -> user text
  const texts = new Map<string, Map<number, string>>() // assistantMessageID -> ordinal -> text
  const toolNames = new Map<string, string>() // tool call id -> tool name
  const toolInputs = new Map<string, unknown>() // tool call id -> input

  const flushStep = (data: any) => {
    const id = data?.assistantMessageID
    const buffer = texts.get(id)
    texts.delete(id)
    if (!buffer) return
    const content = [...buffer.entries()].sort(([a], [b]) => a - b).map(([, text]) => text).filter(Boolean).join("\n")
    ingest(data.sessionID, { role: "assistant", content, external_id: id })
  }

  const flushTool = (data: any) => {
    const id = data?.id
    if (typeof id !== "string" || !id) return
    const name = toolNames.get(id) ?? "tool"
    const input = toolInputs.get(id)
    toolNames.delete(id)
    toolInputs.delete(id)
    const content = `[tool ${name}]\ninput: ${safeJSON(input)}\noutput: ${toolOutput(data)}`
    ingest(data.sessionID, { role: "tool", content, external_id: id, tool_name: name })
  }

  const handlers: Record<string, (data: any) => void> = {
    "session.inbox.enqueued": (data) => {
      const item = data?.item
      if (item?.type !== "user" || typeof item.payload?.text !== "string") return
      bound(prompts)
      prompts.set(data.inboxID, item.payload.text)
    },
    "session.inbox.delivered": (data) => {
      const text = prompts.get(data?.inboxID)
      prompts.delete(data?.inboxID)
      if (text) ingest(data.sessionID, { role: "user", content: text, external_id: data.inboxID })
    },
    "session.inbox.cancelled": (data) => void prompts.delete(data?.inboxID),
    "session.text.ended": (data) => {
      if (typeof data?.assistantMessageID !== "string" || typeof data.text !== "string") return
      bound(texts)
      const buffer = texts.get(data.assistantMessageID) ?? new Map<number, string>()
      buffer.set(Number(data.ordinal) || 0, data.text)
      texts.set(data.assistantMessageID, buffer)
    },
    "session.step.ended": flushStep,
    "session.step.failed": flushStep,
    "session.tool.input.started": (data) => {
      if (typeof data?.id !== "string" || typeof data.name !== "string") return
      bound(toolNames)
      toolNames.set(data.id, data.name)
    },
    "session.tool.called": (data) => {
      if (typeof data?.id !== "string") return
      bound(toolInputs)
      toolInputs.set(data.id, data.input)
    },
    "session.tool.success": flushTool,
    "session.tool.failed": flushTool,
  }

  return (event: any): void => {
    try {
      if (event?.location?.directory !== directory || typeof event.type !== "string") return
      handlers[event.type]?.(event.data)
    } catch {
      /* best-effort: never break the agent */
    }
  }
}

// consumeEvents feeds the event stream to handle until signal aborts. The stream
// fails on overflow or disconnect, so it reconnects with capped exponential
// backoff, giving up after MAX_CONSECUTIVE_RECONNECTS failures without an event.
async function consumeEvents(events: any, signal: AbortSignal, handle: (event: any) => void): Promise<void> {
  let failures = 0
  while (!signal.aborted && failures < MAX_CONSECUTIVE_RECONNECTS) {
    try {
      for await (const event of events.subscribe({ signal })) {
        failures = 0
        handle(event)
      }
    } catch {
      /* stream overflowed or disconnected: reconnect below */
    }
    if (signal.aborted) return
    failures += 1
    await delay(Math.min(RECONNECT_CAP_MS, RECONNECT_BASE_MS * 2 ** (failures - 1)), signal)
  }
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

async function loadContextPlan(dir: string, sessionID: string, prompt: string): Promise<ContextPlan | undefined> {
  const out = await runAcm(dir, ["opencode-context"], JSON.stringify({ session_id: sessionID, prompt }))
  if (out === undefined) return undefined
  try {
    const plan = JSON.parse(out)
    return validPlan(plan) ? plan : undefined
  } catch {
    return undefined
  }
}

function latestUserIndex(messages: any[]): number {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index]?.role === "user") return index
  }
  return -1
}

function userPrompt(message: any): string {
  if (!Array.isArray(message?.content) || message.content.length > MAX_PARTS_PER_MESSAGE) return ""
  return message.content
    .filter((part: any) => part?.type === "text" && typeof part.text === "string" && !acmPart(part))
    .map((part: any) => part.text)
    .join("\n")
}

// messageKey is the acm external_id for a provider message: the message id for
// user/assistant turns, or the tool call id for a tool-result message.
function messageKey(message: any): string | undefined {
  if (typeof message?.id === "string") return message.id
  const result = message?.content?.find?.((part: any) => part?.type === "tool-result")
  return typeof result?.id === "string" ? result.id : undefined
}

function archivePart(part: any, placeholder: string): any | undefined {
  if (acmPart(part)) return part
  if (part?.type === "text") return { ...part, text: placeholder }
  if (part?.type === "tool-result") return { ...part, result: { type: "text", value: placeholder } }
  if (part?.type === "media") return { type: "text", text: placeholder }
  // Older reasoning may carry provider signatures; omit it rather than rewrite it.
  if (part?.type === "reasoning") return undefined
  return part
}

// archiveMessage replaces a message's content with the archive placeholder,
// keeping tool-call/tool-result pairing intact. Messages carrying provider
// `native` replay data are left untouched.
function archiveMessage(message: any, placeholder: string): boolean {
  if (!Array.isArray(message?.content) || message.content.length > MAX_PARTS_PER_MESSAGE || message.native) return false
  const content = message.content.map((part: any) => archivePart(part, placeholder)).filter(Boolean)
  message.content = content.length > 0 ? content : [{ type: "text", text: placeholder }]
  return true
}

function contextPart(kind: string, text: string): Record<string, unknown> {
  return { type: "text", text, metadata: { acmContext: kind } }
}

function applyPlan(messages: any[], anchorIndex: number, plan: ContextPlan): void {
  const anchor = messages[anchorIndex]
  const fresh = new Set(plan.fresh_message_ids)
  const recentStart = Math.min(anchorIndex, Math.max(0, messages.length - plan.fresh_tail_messages))
  let archived = 0
  for (let index = 0; plan.archive_placeholder && index < recentStart; index += 1) {
    const key = messageKey(messages[index])
    if (key !== undefined && fresh.has(key)) continue
    if (archiveMessage(messages[index], plan.archive_placeholder)) archived += 1
  }
  const content = (anchor.content ?? []).filter((part: any) => !acmPart(part))
  if (plan.recall_text) content.push(contextPart("retrieved-context", plan.recall_text))
  if (plan.summary_text && archived > 0) content.push(contextPart("archive-summary", plan.summary_text))
  anchor.content = content
}

function addSystemText(system: any, text: string): void {
  if (!Array.isArray(system) || system.some((part: any) => part?.text === text)) return
  system.push({ type: "text", text })
}

// createContextHooks returns the session `context` and `compaction` hooks for
// one location. The context hook ingests the current user message itself (its id
// is the inbox id the event path uses, so the two are idempotent) because it can
// run before the event stream delivers that prompt, then waits for queued
// ingestion so the plan sees the latest turns.
function createContextHooks(directory: string, queue: Queue, ingest: Sink) {
  const lastPlan = new Map<string, ContextPlan>()

  const context = async (event: any): Promise<void> => {
    try {
      addSystemText(event?.system, SYSTEM_HINT)
      const messages = event?.messages
      if (!Array.isArray(messages) || messages.length === 0 || messages.length > MAX_TRANSFORM_MESSAGES) return
      const anchorIndex = latestUserIndex(messages)
      if (anchorIndex < 0 || typeof event.sessionID !== "string") return
      const prompt = userPrompt(messages[anchorIndex])
      const anchorID = messages[anchorIndex].id
      if (typeof anchorID === "string") ingest(event.sessionID, { role: "user", content: prompt, external_id: anchorID })
      await queue.drain()
      const plan = await loadContextPlan(directory, event.sessionID, prompt)
      if (!plan) return
      bound(lastPlan)
      lastPlan.set(event.sessionID, plan)
      applyPlan(messages, anchorIndex, plan)
    } catch {
      /* best-effort: never break the agent */
    }
  }

  const compaction = async (event: any): Promise<void> => {
    try {
      const sessionID = event?.sessionID
      if (typeof sessionID !== "string") return
      const plan = lastPlan.get(sessionID) ?? (await loadContextPlan(directory, sessionID, ""))
      if (plan?.resume_note) addSystemText(event.system, plan.resume_note)
    } catch {
      /* best-effort */
    }
  }

  return { context, compaction }
}

export default {
  id: "acm",
  setup: async (ctx: any) => {
    const directory = ctx?.location?.directory
    if (typeof directory !== "string" || directory === "") return
    const queue = createQueue()
    const ingest = createSink(directory, queue)
    const controller = new AbortController()
    const hooks = createContextHooks(directory, queue, ingest)
    await ctx.session.hook("context", hooks.context)
    await ctx.session.hook("compaction", hooks.compaction)
    void consumeEvents(ctx.event, controller.signal, createCapture(directory, ingest))
    return () => controller.abort()
  },
}
