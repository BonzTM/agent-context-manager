#!/bin/bash
# PreToolUse hook: block Edit/Write/NotebookEdit unless an ACM receipt
# marker exists for this session.
#
# Marker is created automatically by the PostToolUse hook
# (acm-receipt-mark.sh) when a successful acm get-context call is detected.

set -euo pipefail

INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty')

if [ -z "$SESSION_ID" ]; then
  exit 0
fi

MARKER="/tmp/.acm-receipt-{{project_id}}-${SESSION_ID}"

if [ -f "$MARKER" ]; then
  exit 0
fi

jq -n '{
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    permissionDecision: "deny",
    permissionDecisionReason: "Edit/Write blocked: no ACM receipt for this session. Run /acm-get <task text> first."
  }
}'
exit 0
