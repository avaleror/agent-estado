#!/bin/sh
# Reescribe los archivos que incluyen el protocolo o el puente de Claude.
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
P="$ROOT/plantillas"

{
  printf '%s\n\n' '# Instrucciones para agentes'
  cat "$P/protocolo.md"
  printf '\n'
  cat "$P/partes/proyecto.md"
} > "$P/AGENTS.md"

{
  printf '%s\n\n' '# Instrucciones para agentes'
  cat "$P/protocolo.md"
  printf '\n'
  cat "$P/partes/este-repo.md"
} > "$ROOT/AGENTS.md"

{
  printf '%s\n\n' '# Claude'
  cat "$P/claude-puente.md"
  printf '\n'
} > "$P/CLAUDE.md"

cp "$P/CLAUDE.md" "$ROOT/CLAUDE.md"
mkdir -p "$ROOT/.cursor/rules"
cp "$P/cursor-estado.mdc" "$ROOT/.cursor/rules/estado.mdc"

echo "Plantillas armadas."
