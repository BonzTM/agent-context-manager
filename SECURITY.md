# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability, please report it privately rather than
opening a public issue. Use GitHub's
[private vulnerability reporting](https://github.com/bonztm/agent-context-manager/security/advisories/new)
for the repository.

Please include enough detail to reproduce the issue. We aim to acknowledge
reports promptly and will keep you informed as we investigate and remediate.

## Scope and design notes

`acm` is a local command-line tool. Its security posture follows from its design:

- **No network surface.** `acm` runs no server and opens no network connections of
  its own. Optional LLM summarization invokes a locally installed, already
  authenticated agent CLI as a subprocess; it does not transmit data anywhere
  else.
- **Local storage.** All data is stored in a per-project `.acm/` directory.
  Treat its contents as sensitive — it contains verbatim conversation history,
  which may include secrets that appeared in prompts or tool output.
- **No bundled credentials.** `acm` holds no API keys. When configured to reuse a
  host agent's model, it relies on that agent's existing authentication.
- **Subprocess execution.** When reusing an agent's model, `acm` executes the
  agent CLI directly without a shell; prompts are passed as data (arguments or
  standard input), not interpolated into a shell command.

## Supported versions

The project is pre-1.0. Security fixes are applied to the latest release.
