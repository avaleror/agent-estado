#!/bin/sh
# Tests for the overto command. They run in temporary directories.
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
BIN="$ROOT/bin/overto"
fail=0

ok() {
  echo "ok: $*"
}

bad() {
  echo "FAIL: $*" >&2
  fail=1
}

assert() {
  if "$@"; then
    ok "$*"
  else
    bad "$*"
  fi
}

assert_file() {
  if [ -f "$1" ]; then
    ok "exists $1"
  else
    bad "missing $1"
  fi
}

block() {
  file=$1
  start=$2
  end=$3
  awk -v start="$start" -v end="$end" '
    $0 == start { printing = 1 }
    printing { print }
    $0 == end { printing = 0 }
  ' "$file"
}

# Embedded copies have to match the source.
sh "$ROOT/tests/build.sh" >/dev/null

if block "$ROOT/templates/AGENTS.md" '<!-- estado-protocolo -->' '<!-- /estado-protocolo -->' | cmp -s - "$ROOT/templates/protocol.md"; then
  ok "protocol inside templates/AGENTS.md"
else
  bad "protocol inside templates/AGENTS.md"
fi

if block "$ROOT/AGENTS.md" '<!-- estado-protocolo -->' '<!-- /estado-protocolo -->' | cmp -s - "$ROOT/templates/protocol.md"; then
  ok "protocol inside AGENTS.md"
else
  bad "protocol inside AGENTS.md"
fi

if block "$ROOT/templates/CLAUDE.md" '<!-- estado-claude -->' '<!-- /estado-claude -->' | cmp -s - "$ROOT/templates/claude-bridge.md"; then
  ok "bridge inside templates/CLAUDE.md"
else
  bad "bridge inside templates/CLAUDE.md"
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# Do not init the home directory.
home="$TMP/home"
mkdir -p "$home"
if (cd "$home" && HOME="$home" "$BIN" init >/dev/null 2>&1); then
  bad "init in HOME should fail"
else
  ok "init in HOME rejected"
fi
if [ -e "$home/AGENTS.md" ]; then
  bad "init in HOME should not create files"
else
  ok "HOME left intact"
fi

# New project.
proj="$TMP/proj"
mkdir -p "$proj"
git -C "$proj" init -q -b main
git -C "$proj" config user.name "Ada Test"
git -C "$proj" config user.email "ada@example.com"
(cd "$proj" && "$BIN" init >/dev/null)

assert_file "$proj/AGENTS.md"
assert_file "$proj/HANDOFF.md"
assert_file "$proj/CLAUDE.md"
assert_file "$proj/.cursor/rules/handoff.mdc"

today=$(date +%Y-%m-%d)
if grep -q "^Updated: $today$" "$proj/HANDOFF.md"; then
  ok "today's date"
else
  bad "today's date"
fi
if grep -q "^Who: Ada Test$" "$proj/HANDOFF.md"; then
  ok "git name"
else
  bad "git name"
fi
if grep -q '^## Failed$' "$proj/HANDOFF.md" && grep -q '^## Files$' "$proj/HANDOFF.md" && grep -q '^## Dropped$' "$proj/HANDOFF.md"; then
  ok "template has failures and files"
else
  bad "template has failures and files"
fi
if grep -q -x '.claude.local.md' "$proj/.gitignore"; then
  ok "local gitignore created"
else
  bad "local gitignore created"
fi

n_proto=$(grep -c -F '<!-- estado-protocolo -->' "$proj/AGENTS.md" || true)
n_claude=$(grep -c -F '<!-- estado-claude -->' "$proj/CLAUDE.md" || true)
if [ "$n_proto" -eq 1 ] && [ "$n_claude" -eq 1 ]; then
  ok "one mark in new files"
else
  bad "duplicate marks in new files ($n_proto, $n_claude)"
fi

# Second run: do not duplicate and do not overwrite HANDOFF.md.
printf '\n- own milestone\n' >> "$proj/HANDOFF.md"
cp "$proj/AGENTS.md" "$proj/AGENTS.before"
cp "$proj/HANDOFF.md" "$proj/HANDOFF.before"
(cd "$proj" && "$BIN" init >/dev/null)
if cmp -s "$proj/AGENTS.before" "$proj/AGENTS.md"; then
  ok "second init does not change AGENTS.md"
else
  bad "second init changed AGENTS.md"
fi
if cmp -s "$proj/HANDOFF.before" "$proj/HANDOFF.md"; then
  ok "second init does not overwrite HANDOFF.md"
else
  bad "second init overwrote HANDOFF.md"
fi
n_local=$(grep -c -F '.claude.local.md' "$proj/.gitignore" || true)
if [ "$n_local" -eq 1 ]; then
  ok "local gitignore not duplicated"
else
  bad "local gitignore duplicated ($n_local)"
fi

# Existing files: keep the prose and replace the old block.
old="$TMP/old"
mkdir -p "$old"
cat > "$old/AGENTS.md" <<'EOF'
# Team rules

Do not delete this paragraph.

<!-- estado-protocolo -->
old text that must go
<!-- /estado-protocolo -->

## Project

This section stays.
EOF
cat > "$old/CLAUDE.md" <<'EOF'
# Preferences

Answer in Spanish.

<!-- estado-claude -->
old bridge
<!-- /estado-claude -->

## Notes

This note stays.
EOF
printf '%s\n' '# Handoff' 'Updated: 1999-01-01' 'PERSONAL DONE' > "$old/HANDOFF.md"

(cd "$old" && "$BIN" init >/dev/null)

if grep -q 'Do not delete this paragraph.' "$old/AGENTS.md" && grep -q 'This section stays.' "$old/AGENTS.md"; then
  ok "own AGENTS.md text kept"
else
  bad "own AGENTS.md text"
fi
if grep -q 'old text that must go' "$old/AGENTS.md"; then
  bad "old protocol still in AGENTS.md"
else
  ok "old protocol replaced"
fi
if block "$old/AGENTS.md" '<!-- estado-protocolo -->' '<!-- /estado-protocolo -->' | cmp -s - "$ROOT/templates/protocol.md"; then
  ok "protocol updated"
else
  bad "protocol updated"
fi
if grep -q 'Answer in Spanish.' "$old/CLAUDE.md" && grep -q 'This note stays.' "$old/CLAUDE.md"; then
  ok "own CLAUDE.md text kept"
else
  bad "own CLAUDE.md text"
fi
if grep -q 'old bridge' "$old/CLAUDE.md"; then
  bad "old bridge still there"
else
  ok "old bridge replaced"
fi
if grep -q 'PERSONAL DONE' "$old/HANDOFF.md" && grep -q '1999-01-01' "$old/HANDOFF.md"; then
  ok "existing HANDOFF.md left intact"
else
  bad "existing HANDOFF.md"
fi

# Old filename: rename once, and leave the prose alone.
legacy="$TMP/legacy"
mkdir -p "$legacy"
printf '%s\n' '# State' 'Actualizado: 1999-01-01' 'KEEP THIS' > "$legacy/ESTADO.md"
(cd "$legacy" && "$BIN" init >/dev/null)
if [ -f "$legacy/HANDOFF.md" ] && [ ! -e "$legacy/ESTADO.md" ] && grep -q 'KEEP THIS' "$legacy/HANDOFF.md" && grep -q '1999-01-01' "$legacy/HANDOFF.md"; then
  ok "ESTADO.md renamed to HANDOFF.md"
else
  bad "ESTADO.md renamed to HANDOFF.md"
fi

# Both names: do not delete either.
both="$TMP/both"
mkdir -p "$both"
printf '%s\n' 'new file' > "$both/HANDOFF.md"
printf '%s\n' 'old file' > "$both/ESTADO.md"
(cd "$both" && "$BIN" init >/dev/null)
if grep -q 'new file' "$both/HANDOFF.md" && grep -q 'old file' "$both/ESTADO.md"; then
  ok "both handoff files left in place"
else
  bad "both handoff files left in place"
fi

# Old Cursor rule is removed because init always rewrote it.
ruled="$TMP/ruled"
mkdir -p "$ruled/.cursor/rules"
printf '%s\n' 'old rule' > "$ruled/.cursor/rules/estado.mdc"
(cd "$ruled" && "$BIN" init >/dev/null)
if [ -f "$ruled/.cursor/rules/handoff.mdc" ] && [ ! -e "$ruled/.cursor/rules/estado.mdc" ]; then
  ok "old cursor rule replaced"
else
  bad "old cursor rule replaced"
fi

# No marks: the block is added once.
plain="$TMP/plain"
mkdir -p "$plain"
printf '%s\n' '# Just mine' > "$plain/AGENTS.md"
printf '%s\n' '# My Claude' > "$plain/CLAUDE.md"
(cd "$plain" && "$BIN" init >/dev/null)
(cd "$plain" && "$BIN" init >/dev/null)
n_proto=$(grep -c -F '<!-- estado-protocolo -->' "$plain/AGENTS.md" || true)
n_claude=$(grep -c -F '<!-- estado-claude -->' "$plain/CLAUDE.md" || true)
if [ "$n_proto" -eq 1 ] && [ "$n_claude" -eq 1 ] && grep -q 'Just mine' "$plain/AGENTS.md" && grep -q 'My Claude' "$plain/CLAUDE.md"; then
  ok "block added once"
else
  bad "block added once ($n_proto, $n_claude)"
fi

# Opening mark without a close: touch nothing.
broken="$TMP/broken"
mkdir -p "$broken"
printf '%s\n' '# Broken' '<!-- estado-protocolo -->' 'no close' > "$broken/AGENTS.md"
cp "$broken/AGENTS.md" "$broken/AGENTS.before"
if (cd "$broken" && "$BIN" init >/dev/null 2>&1); then
  bad "init with a broken mark should fail"
else
  ok "init with a broken mark rejected"
fi
if cmp -s "$broken/AGENTS.before" "$broken/AGENTS.md" && [ ! -e "$broken/HANDOFF.md" ]; then
  ok "broken mark writes nothing"
else
  bad "broken mark wrote or changed files"
fi

# show and date.
if (cd "$proj" && "$BIN" show) | grep -q 'own milestone'; then
  ok "show prints HANDOFF.md"
else
  bad "show"
fi
(cd "$proj" && "$BIN" date >/dev/null)
if grep -q "^Updated: $today$" "$proj/HANDOFF.md" && grep -q 'own milestone' "$proj/HANDOFF.md"; then
  ok "date changes the day and keeps the rest"
else
  bad "date"
fi

# Old label stays when that is the label the file already uses.
es="$TMP/es"
mkdir -p "$es"
printf '%s\n' '# State' 'Actualizado: 1999-01-01' 'note' > "$es/HANDOFF.md"
(cd "$es" && "$BIN" fecha >/dev/null)
if grep -q "^Actualizado: $today$" "$es/HANDOFF.md" && grep -q '^note$' "$es/HANDOFF.md"; then
  ok "date keeps the Actualizado label"
else
  bad "date keeps the Actualizado label"
fi

ign="$TMP/ign"
mkdir -p "$ign"
printf '*.log' > "$ign/.gitignore"
(cd "$ign" && "$BIN" init >/dev/null)
(cd "$ign" && "$BIN" init >/dev/null)
n_local=$(grep -c -F '.claude.local.md' "$ign/.gitignore" || true)
if [ "$n_local" -eq 1 ] && grep -q '^\*\.log$' "$ign/.gitignore"; then
  ok "existing gitignore keeps its rules"
else
  bad "existing gitignore ($n_local)"
fi

# Unknown command.
if "$BIN" does-not-exist >/dev/null 2>&1; then
  bad "unknown command should fail"
else
  ok "unknown command rejected"
fi

# The command still finds the templates when invoked through a symlink.
linked="$TMP/bin"
mkdir -p "$linked"
ln -s "$BIN" "$linked/overto"
via="$TMP/via"
mkdir -p "$via"
(cd "$via" && PATH="$linked:$PATH" overto init >/dev/null)
assert_file "$via/AGENTS.md"
assert_file "$via/HANDOFF.md"

# share prints a note for one coworker, and skips the empty template lines.
note=$(cd "$proj" && "$BIN" share Ada)
if printf '%s\n' "$note" | grep -q 'Over to you, Ada.' \
  && printf '%s\n' "$note" | grep -q '^- HANDOFF.md$' \
  && printf '%s\n' "$note" | grep -q 'For your AI:' \
  && printf '%s\n' "$note" | grep -q 'Read HANDOFF.md, follow AGENTS.md' \
  && ! printf '%s\n' "$note" | grep -q 'One sentence: what we are doing.'; then
  ok "share names the coworker and the files"
else
  bad "share names the coworker and the files"
fi
awk '
  /One sentence: what we are doing./ { print "Ship the lab."; next }
  { print }
' "$proj/HANDOFF.md" > "$proj/HANDOFF.new"
mv "$proj/HANDOFF.new" "$proj/HANDOFF.md"
if (cd "$proj" && "$BIN" share) | grep -q 'Goal: Ship the lab.'; then
  ok "share includes the goal"
else
  bad "share includes the goal"
fi

if [ "$fail" -ne 0 ]; then
  echo "Some tests failed." >&2
  exit 1
fi

echo "All tests passed."
