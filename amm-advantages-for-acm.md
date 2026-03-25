# AMM → ACM Retrofit Opportunities

This document captures patterns, features, and architectural decisions from AMM (Agent Memory Manager) that ACM (Agent Context Manager) could adopt to improve consistency and functionality.

## 1. Ultra-Thin Main Pattern

**AMM Pattern:**
- `cmd/amm/main.go`: 15 lines - delegates ALL routing to `internal/adapters/cli/`
- `cmd/amm-mcp/main.go`: 15 lines - delegates to `internal/adapters/mcp/`

**ACM Current:**
- `cmd/acm/main.go`: 307 lines with command routing logic
- `cmd/acm-mcp/main.go`: 275 lines with tool dispatch

**Retrofit Instructions:**
1. Move all command routing from `cmd/acm/main.go` to `internal/adapters/cli/runner.go`
2. Create `cli.Run(args []string) error` function as single entry point
3. Simplify main.go to:
   ```go
   func main() {
       if err := cli.Run(os.Args[1:]); err != nil {
           fmt.Fprintf(os.Stderr, "acm: %v\n", err)
           os.Exit(1)
       }
   }
   ```
4. Move MCP tool dispatch from main to `internal/adapters/mcp/server.go`

## 2. JSON-RPC 2.0 MCP Protocol

**AMM Pattern:**
- Implements official MCP spec: `initialize`, `tools/list`, `tools/call`
- JSON-RPC 2.0 over stdio with proper request/response structure
- Protocol version negotiation

**ACM Current:**
- Custom subcommand-based: `acm-mcp tools`, `acm-mcp invoke --tool <name>`
- Simpler but non-standard

**Retrofit Instructions:**
1. Replace `acm-mcp tools` and `acm-mcp invoke` with JSON-RPC server
2. Implement `initialize` method returning capabilities
3. Implement `tools/list` returning tool definitions
4. Implement `tools/call` for tool invocation
5. Maintain backward compatibility or deprecate old pattern

## 3. Separate CLI/MCP Reference Documentation

**AMM Pattern:**
- `docs/cli-reference.md`: Complete CLI command reference
- `docs/mcp-reference.md`: Complete MCP tool reference
- README links to both instead of duplicating

**ACM Current:**
- CLI reference embedded in README (lines 254-313)
- MCP reference embedded in README (lines 229-252)

**Retrofit Instructions:**
1. Extract CLI reference from README to `docs/cli-reference.md`
2. Extract MCP reference from README to `docs/mcp-reference.md`
3. Update README to link to these docs
4. Add navigation structure: "See [CLI Reference](docs/cli-reference.md)"

## 4. Integration Guide Architecture

**AMM Pattern:**
- `docs/integration.md`: Generic integration model
- `docs/{runtime}-integration.md`: Runtime-specific guides (codex, opencode, openclaw, hermes)
- Clear separation between shared concepts and runtime specifics

**ACM Current:**
- Integration guidance embedded in README
- Some runtime-specific content in `skills/acm-broker/`

**Retrofit Instructions:**
1. Create `docs/integration.md` with generic integration patterns
2. Create separate files for each runtime integration
3. Move runtime-specific setup from README to these guides
4. Keep README high-level with links to deep dives

## 5. Architecture Documentation Depth

**AMM Pattern:**
- `docs/architecture.md`: Comprehensive architecture reference
- Memory layers clearly defined (A-E)
- Service layer as single entry point documented
- Adapter pattern clearly explained

**ACM Current:**
- Architecture diagrams exist but less textual depth
- Internal patterns documented in AGENTS.md

**Retrofit Instructions:**
1. Expand `docs/architecture.md` or create if missing
2. Document layer boundaries clearly
3. Explain data flow: CLI/MCP → Service → Repository
4. Add component interaction diagrams

## 6. Command Catalog Pattern

**AMM Pattern:**
- `internal/contracts/v1/commands.go`: Centralized command registry
- `CommandRegistry` map with metadata
- `CommandInfo` struct with name + description

**ACM Current:**
- Commands defined in `internal/contracts/v1/command_catalog.go`
- Similar pattern but could be more discoverable

**Retrofit Instructions:**
1. Ensure all commands are in catalog with descriptions
2. Use catalog for help text generation
3. Keep catalog as source of truth

## 7. Schema-First Design (Partial)

**AMM Pattern:**
- Input schemas defined in MCP server code
- Strong typing throughout
- Could benefit from formal JSON schemas (see below)

**ACM Current:**
- `spec/v1/` directory with formal JSON schemas
- Schema validation in tests

**Retrofit Instructions for AMM:**
1. AMM should adopt ACM's `spec/v1/` pattern
2. Create formal JSON schemas for all commands
3. Add schema validation tests

## 8. Ultra-Minimal Binary Entrypoints

**AMM Pattern:**
```go
// cmd/amm/main.go
package main

import (
    "fmt"
    "os"
    "github.com/bonztm/agent-memory-manager/internal/adapters/cli"
)

func main() {
    if err := cli.Run(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "amm: %v\n", err)
        os.Exit(1)
    }
}
```

**Benefits:**
- Zero business logic in main
- Easy to test (cli.Run is testable)
- No globals or initialization side effects
- Clear separation of concerns

**Retrofit Instructions:**
1. Strip main.go files to minimum
2. Move all routing to adapters
3. Ensure adapters are pure functions where possible

## 9. Consistent Error Envelope Pattern

**AMM Pattern:**
```go
type Envelope struct {
    OK        bool        `json:"ok"`
    Command   string      `json:"command"`
    Timestamp string      `json:"timestamp"`
    Result    interface{} `json:"result,omitempty"`
    Error     *EnvError   `json:"error,omitempty"`
}
```
- Used for ALL CLI output
- Consistent structure across commands
- Timestamp included

**ACM Current:**
- Similar envelope but not consistently applied
- Some commands return raw results

**Retrofit Instructions:**
1. Standardize on envelope pattern for all commands
2. Include timestamp in all responses
3. Ensure error codes are consistent

## 10. Runtime-Specific Integration Guides

**AMM Pattern:**
Each runtime gets its own focused guide:
- `docs/codex-integration.md`: MCP + hooks + AGENTS
- `docs/opencode-integration.md`: MCP + plugin glue
- `docs/openclaw-integration.md`: Real examples + native hooks
- `docs/hermes-agent-integration.md`: Sidecar model

Each guide includes:
- Prerequisites
- Step-by-step setup
- Configuration examples
- Troubleshooting

**Retrofit Instructions:**
1. Split README's "Agent Integration" section into separate files
2. Add Codex guide (beyond just skill pack mention)
3. Add OpenCode guide (detailed, not just repo-local docs)
4. Each guide should be standalone

---

## Implementation Priority

**High (Quick Wins):**
1. Ultra-thin main pattern (#1, #8)
2. Separate CLI/MCP docs (#3)
3. Consistent error envelopes (#9)

**Medium (Structural):**
4. JSON-RPC MCP protocol (#2)
5. Integration guide architecture (#4, #10)
6. Command catalog improvements (#6)

**Low (Nice to Have):**
7. Architecture documentation depth (#5)

---

## Notes

- AMM and ACM serve different purposes but should share interface patterns
- AMM focuses on memory substrate; ACM focuses on task workflow
- Cross-pollination makes both projects more consistent and maintainable
- Consider creating shared `agent-toolkit` patterns for common structures
