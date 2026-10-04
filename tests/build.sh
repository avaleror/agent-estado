#!/bin/sh
# Rewrite the files that include the protocol or the Claude bridge.
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
P="$ROOT/templates"

{
  printf '%s\n\n' '# Agent instructions'
  cat "$P/protocol.md"
  printf '\n'
  cat "$P/parts/project.md"
} > "$P/AGENTS.md"

{
  printf '%s\n\n' '# Agent instructions'
  cat "$P/protocol.md"
  printf '\n'
  cat "$P/parts/this-repo.md"
} > "$ROOT/AGENTS.md"

{
  printf '%s\n\n' '# Claude'
  cat "$P/claude-bridge.md"
  printf '\n'
} > "$P/CLAUDE.md"

cp "$P/CLAUDE.md" "$ROOT/CLAUDE.md"
mkdir -p "$ROOT/.cursor/rules"
cp "$P/cursor-handoff.mdc" "$ROOT/.cursor/rules/handoff.mdc"

echo "Templates built."
