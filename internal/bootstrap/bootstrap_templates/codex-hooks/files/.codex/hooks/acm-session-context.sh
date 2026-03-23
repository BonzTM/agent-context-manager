#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/acm-common.sh"

ensure_jq
INPUT=$(cat)
HOOK_EVENT=$(json_get '.hook_event_name // empty')
SESSION_ID=$(json_get '.session_id // empty')

if [ "$HOOK_EVENT" != "SessionStart" ] || [ -z "$SESSION_ID" ]; then
  exit 0
fi

prepare_state_dir "$SESSION_ID"
MESSAGE=""

append_line "Repo contract reminder: read AGENTS.md first. The repo-root AGENTS.md stays authoritative over Codex companion files."
append_line "Before implementation or debugging work, start with acm context for the current task so ACM can return the active rules, memories, and plan state."
append_line "When work becomes multi-step or governed scope expands, update acm work and declare plan.discovered_paths before expecting review or done to pass."
append_line "Before closing verify-sensitive work, run acm verify, then acm review --run when the workflow requires it, then acm done. Save reusable decisions with acm memory."

if has_state change_prompt_seen && ! has_state done_prompt_seen; then
  append_line "This session has seen implementation-style prompts without an explicit acm done closeout marker yet."
fi

emit_additional_context "SessionStart" "$MESSAGE"
