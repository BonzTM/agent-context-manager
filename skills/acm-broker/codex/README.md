# Codex Companion - acm-broker

This folder provides Codex-first companion docs for repos that use ACM.
Codex already uses the installed `acm-broker` skill through its global `SKILL.md`; these companion files make the repo-local operating model explicit without pretending Codex has Claude-style slash commands or process hooks.

## Recommended setup

1. Install the global Codex skill:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bonztm/agent-context-manager/main/scripts/install-skill-pack.sh) --codex
```

2. Keep the repo-root `AGENTS.md` as the source of truth for the project workflow.

3. Optionally seed repo-local Codex companion files:

```bash
acm init --apply-template codex-pack
```

That template seeds:

- `.codex/acm-broker/README.md`
- `.codex/acm-broker/AGENTS.example.md`

The template is documentation only. It does not add hidden hooks or special Codex-only runtime behavior.

## Recommended Codex loop

Codex can drive the full ACM workflow directly:

1. `acm context`
2. `acm fetch` only when you need to hydrate specific plan, task, memory, or pointer content
3. `acm work` for multi-step tasks or when governed file scope expands through `plan.discovered_paths`
4. `acm review` when a workflow gate needs a review record or runnable review
5. `acm verify`
6. `acm done`
7. `acm memory`

Use the same maintenance loop as any other primary ACM operator when rules, tags, tests, workflows, onboarding, or tool-surface behavior change:

- `acm sync --mode working_tree --insert-new-candidates`
- `acm health --include-details`

## Scope and closeout notes

- `context` is task framing, not a substitute for Codex reading the repo itself.
- If governed work discovers later-relevant files, declare them with `work.plan.discovered_paths` before expecting `review` or `done` to pass.
- `done` can omit `files_changed` and rely on the receipt baseline delta when that is more convenient.

## AGENTS companion

Use [AGENTS.example.md](AGENTS.example.md) as the Codex-oriented companion example for repo-root `AGENTS.md` contracts.
It is intentionally thin: the repo-root `AGENTS.md` stays authoritative, and the installed skill plus this companion should map back to that file rather than inventing a second workflow.
