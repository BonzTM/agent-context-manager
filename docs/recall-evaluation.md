# Recall evaluation

Automatic recall is gated by the anonymized fixture corpus in
`internal/agents/testdata/recall_corpus.json`. The corpus fixes its clock and
records candidates, relevant IDs, and exact expected top-k order for decisions,
identifiers, user requirements, noisy tool output, resumed sessions,
summary-only terms, and active-versus-historical summaries. Its expected
results cover system, user, assistant, and tool messages plus summaries.

`TestRecallCorpusBaseline` reports macro-average Recall@k and mean reciprocal
rank (MRR), requires both to remain at the committed 1.000 baseline, checks the
exact top-k list for every case, and runs each ranking twice to detect
nondeterminism. Run it directly with:

```sh
go test ./internal/agents -run TestRecallCorpusBaseline -count=1 -v
```

The production path remains bounded:

- prompt extraction keeps at most 12 distinct salient terms;
- the default five-item recall searches at most 40 messages and 10 summaries;
- all flag values cap combined candidates at 50 and injected results at 10;
- at most two injected results can be summaries;
- each injected snippet is capped at 300 runes; and
- fresh-tail detection reads at most 4,096 small identity/token rows and excludes
  the current conversation's protected non-tool messages before ranking.

Summary candidates include both nodes referenced by the active window and
historical DAG nodes. Active status is a deterministic ranking signal, not a
search filter. Message hits show `acm describe <msg-id>` guidance; summary hits
show `acm expand <sum-id>` guidance.

The committed baseline does not show a measurable retrieval gap that warrants
query variants or scope escalation. Add representative failing cases and
record the pre-change metrics before introducing either mechanism.
