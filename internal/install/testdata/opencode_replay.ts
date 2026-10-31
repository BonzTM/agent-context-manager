// Replay harness for the embedded OpenCode plugin (run by opencode_plugin_test.go
// under Bun). It loads the plugin with a fake OpenCode 2.x context, streams the
// recorded events, runs the session context and compaction hooks once, and
// prints the resulting request to stdout as JSON. The fake `acm` on PATH records
// every invocation for the Go test to assert on.
import { readFileSync } from "node:fs"

const [pluginPath, eventsPath, directory] = process.argv.slice(2)
const events = readFileSync(eventsPath, "utf8")
  .split("\n")
  .filter(Boolean)
  .map((line) => JSON.parse(line.replaceAll("__DIR__", directory)))

const hooks: Record<string, (event: any) => Promise<void> | void> = {}
let delivered!: () => void
const allDelivered = new Promise<void>((resolve) => (delivered = resolve))

async function* subscribe(options: { signal: AbortSignal }) {
  for (const event of events) yield event
  delivered()
  // Hold the stream open like a live server until the plugin unloads.
  await new Promise((resolve) => options.signal.addEventListener("abort", resolve, { once: true }))
}

const ctx = {
  location: { directory },
  event: { subscribe },
  session: { hook: async (name: string, callback: any) => void (hooks[name] = callback) },
}

const plugin = (await import(pluginPath)).default
const cleanup = await plugin.setup(ctx)
await allDelivered

const text = (value: string) => ({ type: "text", text: value })
const request = {
  sessionID: "ses_test",
  system: [text("base system")],
  messages: [
    { id: "msg_old", role: "user", content: [text("old question")] },
    { id: "msg_old_fresh", role: "assistant", content: [text("kept verbatim")] },
    {
      id: "msg_a1",
      role: "assistant",
      content: [
        { type: "reasoning", text: "private chain", providerMetadata: { anthropic: { signature: "sig" } } },
        { type: "tool-call", id: "call_1", name: "read", input: { path: "a.txt" } },
      ],
    },
    { role: "tool", content: [{ type: "tool-result", id: "call_1", name: "read", result: { type: "text", value: "big file" } }] },
    { id: "msg_a2", role: "assistant", content: [text("recent answer")] },
    { id: "msg_new", role: "user", content: [text("new question"), { type: "text", text: "stale", metadata: { acmContext: "retrieved-context" } }] },
  ],
}
await hooks.context(request)
const compaction = { sessionID: "ses_test", system: [] as any[] }
await hooks.compaction(compaction)
await cleanup?.()
console.log(JSON.stringify({ request, compaction }))
