#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Install acm-broker skill assets for Codex and Claude.

Usage:
  scripts/install-skill-pack.sh [options]

Options:
  --codex-home <path>     Codex home directory (default: $CODEX_HOME or ~/.codex)
  --claude-target <path>  Target repo for Claude commands (default: current directory)
  --skip-codex            Skip Codex skill install
  --skip-claude           Skip Claude command-pack install
  -h, --help              Show help

Examples:
  scripts/install-skill-pack.sh
  scripts/install-skill-pack.sh --claude-target /path/to/project
  scripts/install-skill-pack.sh --skip-claude
USAGE
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

codex_home="${CODEX_HOME:-${HOME}/.codex}"
claude_target="$(pwd)"
install_codex=true
install_claude=true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --codex-home)
      [[ $# -ge 2 ]] || { echo "error: --codex-home requires a value" >&2; exit 2; }
      codex_home="$2"
      shift 2
      ;;
    --claude-target)
      [[ $# -ge 2 ]] || { echo "error: --claude-target requires a value" >&2; exit 2; }
      claude_target="$2"
      shift 2
      ;;
    --skip-codex)
      install_codex=false
      shift
      ;;
    --skip-claude)
      install_claude=false
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "${install_codex}" == false && "${install_claude}" == false ]]; then
  echo "error: both install targets are disabled; remove --skip-codex or --skip-claude" >&2
  exit 2
fi

skill_src="${REPO_ROOT}/skills/acm-broker"
if [[ ! -d "${skill_src}" ]]; then
  echo "error: skill source directory not found: ${skill_src}" >&2
  exit 1
fi

if [[ "${install_codex}" == true ]]; then
  codex_skills_dir="${codex_home}/skills"
  codex_target="${codex_skills_dir}/acm-broker"
  mkdir -p "${codex_skills_dir}"
  rm -rf "${codex_target}"
  cp -R "${skill_src}" "${codex_target}"
  echo "Installed Codex skill: ${codex_target}"
fi

if [[ "${install_claude}" == true ]]; then
  claude_target="$(cd "${claude_target}" && pwd)"
  claude_dir="${claude_target}/.claude"
  claude_commands_dir="${claude_dir}/commands"
  claude_pack_dir="${claude_dir}/acm-broker"

  mkdir -p "${claude_commands_dir}" "${claude_pack_dir}"
  cp "${skill_src}/claude/commands/"*.md "${claude_commands_dir}/"
  cp "${skill_src}/claude/CLAUDE.md" "${claude_pack_dir}/CLAUDE.md"
  cp "${skill_src}/claude/README.md" "${claude_pack_dir}/README.md"

  echo "Installed Claude commands: ${claude_commands_dir}"
  echo "Installed Claude companion docs: ${claude_pack_dir}"
fi

echo "Done. Restart Codex and/or Claude Code to load the installed assets."
