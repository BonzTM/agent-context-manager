# Session lifecycle

## Capture modes

`.acm-policy.toml` supports two no-storage session modes:

```toml
ignore_sessions = ["healthcheck-*", "throwaway-*"]
stateless_sessions = ["review-only-*"]
```

Ignored sessions receive neither automatic recall nor writes. Stateless sessions
receive automatic recall and drill-down but create no conversation/message rows.
If patterns overlap, ignore wins. The older `exclude_sessions` key remains a
compatibility alias for `stateless_sessions`.

## Pins

`acm pin <conversation-id>` permanently exempts a conversation from retention
pruning. Carry-over pins its source automatically because the target seed
contains resolvable `sum_` pointers into that source.

## Retention pruning

`acm prune` is dry-run by default and selects conversations inactive for 30 days:

```sh
acm prune
acm prune --older-than 2160h
acm prune --older-than 720h --apply
```

The plan labels each conversation `eligible`, `pinned`, or `unexpanded`.
Successful `acm expand` and `acm expand-query` calls acknowledge the requested
summary root. A conversation containing any never-acknowledged summary is not
eligible unless `--force` is explicit.

Apply mode creates an owner-only `pre-prune-*.db` snapshot through SQLite
`VACUUM INTO`, reopens it, and requires integrity/FTS parity before beginning the
bounded delete transaction. It removes message and summary FTS rows explicitly,
then relies on foreign-key cascades for the conversation DAG. Offloaded files are
removed after commit; a cleanup error names the verified backup needed for
rollback.

To roll back, stop commands/hooks using the project, preserve the failed
`.acm/acm.db`, replace it with the reported `pre-prune-*.db`, enforce mode
`0600`, and run `acm doctor` before resuming capture. ACM has no daemon, but host
agent hooks can otherwise race a manual restore.

## Carry-over

Seed a new session from the deepest available summary layer:

```sh
acm carry-over <source-conversation> <target-session>
acm carry-over <source-conversation> <target-session> --agent codex --depth 1
```

The selected layer is chronological and bounded to 16 summaries by default
(maximum 64) and 60,000 rendered characters. The target receives one system
message containing summary pointers and content. Its external identity includes
the source and depth, so rerunning the command deduplicates exactly.
