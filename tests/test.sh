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

# A secret-looking line must not be pasted with the note.
sec="$TMP/sec"
mkdir -p "$sec"
printf '%s\n' '# Handoff' 'Updated: 2026-10-05' '' '## Goal' '' 'sk-abcdefghijklmnopqrst' '' '## Next' '' 'Ship it.' > "$sec/HANDOFF.md"
note=$(cd "$sec" && "$BIN" share Ada 2>"$sec/err") || true
if printf '%s\n' "$note" | grep -q 'sk-abcdefghijklmnopqrst' || printf '%s\n' "$note" | grep -q '^Goal:' || printf '%s\n' "$note" | grep -q '^Next:'; then
  bad "share leaked Goal or Next"
elif grep -q 'looks like it contains a secret' "$sec/err" && printf '%s\n' "$note" | grep -q 'Over to you, Ada.'; then
  ok "share omits Goal and Next when a secret is present"
else
  bad "share secret warning"
fi

# here and to. The fake peer never opens a socket.
FAKE="$ROOT/tests/fake-pass.sh"
chmod +x "$FAKE"
ID='7k9qm3hf-4rv2-np8c-wxt3-f6dt-aj5m-eyb2qr'
printf '%s\n' "$ID" > "$TMP/idfile"
recv_pid=
BOX=$TMP/box
trap 'if [ -n "${recv_pid:-}" ]; then kill "$recv_pid" 2>/dev/null || true; wait "$recv_pid" 2>/dev/null || true; fi; rm -rf "$TMP"' EXIT

mode_of() {
  case $(uname -s) in
    Darwin) stat -f %Lp "$1" ;;
    *) stat -c %a "$1" ;;
  esac
}

fail_cmd() {
  want=$1
  needle=$2
  shift 2
  set +e
  "$@" >"$TMP/cmd-out" 2>"$TMP/cmd-err"
  got=$?
  set -e
  if [ "$got" -eq "$want" ] && grep -F -q -- "$needle" "$TMP/cmd-err"; then
    ok "$needle"
  else
    bad "$needle (exit $got)"
    echo "--- stderr ---" >&2
    cat "$TMP/cmd-err" >&2 || true
    echo "--- stdout ---" >&2
    cat "$TMP/cmd-out" >&2 || true
  fi
}

reset_box() {
  rm -rf "$BOX"
  mkdir -p "$BOX"
}

start_here() {
  dir=$1
  ans=$2
  shift 2
  printf '%s' "$ans" > "$TMP/here-ans"
  reset_box
  (
    cd "$dir" || exit 1
    exec env -u OVERTO_INTRO -u OVERTO_ID_FILE -u OVERTO_FAKE \
      OVERTO_PASS="$FAKE" \
      OVERTO_FAKE_BOX="$BOX" \
      "$BIN" here --for 1 --direct 127.0.0.1:9 "$@"
  ) <"$TMP/here-ans" >"$TMP/here-out" 2>"$TMP/here-err" &
  recv_pid=$!
  i=0
  while [ ! -f "$BOX/ready" ]; do
    i=$((i + 1))
    if [ "$i" -gt 20 ]; then
      echo "receiver did not become ready" >&2
      return 1
    fi
    sleep 1
  done
}

run_to() {
  dir=$1
  ans=$2
  shift 2
  printf '%s' "$ans" > "$TMP/to-ans"
  set +e
  (
    cd "$dir" || exit 1
    exec env -u OVERTO_INTRO -u OVERTO_FAKE \
      OVERTO_PASS="$FAKE" \
      OVERTO_FAKE_BOX="$BOX" \
      OVERTO_ID_FILE="$TMP/idfile" \
      "$BIN" to --direct 127.0.0.1:9 "$@"
  ) <"$TMP/to-ans" >"$TMP/to-out" 2>"$TMP/to-err"
  to_rc=$?
  set -e
}

finish_here() {
  set +e
  wait "$recv_pid"
  here_rc=$?
  set -e
  recv_pid=
}

if "$BIN" version | grep -q '^3$'; then
  ok "version is 3"
else
  bad "version is 3"
fi

if "$BIN" help | grep -F -q 'overto here' && "$BIN" help | grep -F -q 'overto to'; then
  ok "help lists here and to"
else
  bad "help lists here and to"
fi

initmsg=$TMP/initmsg
mkdir -p "$initmsg"
if (cd "$initmsg" && "$BIN" init) | grep -F -q 'they run overto here, and you run overto to with the id.'; then
  ok "init mentions here and to"
else
  bad "init mentions here and to"
fi

fail_cmd 1 "--for takes a whole number of minutes." "$BIN" here --for 30m
fail_cmd 1 "--for takes a whole number of minutes." "$BIN" here --for

plain=$TMP/plain
mkdir -p "$plain"
printf 'hello\n' > "$plain/HANDOFF.md"

# cwd matters for the place check, so run here and to from inside a project.
run_in() {
  dir=$1
  shift
  sh -c 'cd "$1" && shift && exec "$@"' sh "$dir" "$@"
}

fail_cmd 1 "no introduction point is set. Pass --intro, or use --direct." \
  run_in "$plain" env -u OVERTO_INTRO -u OVERTO_PASS "$BIN" here
fail_cmd 1 "the introduction point has to be https." \
  run_in "$plain" env -u OVERTO_PASS "$BIN" here --intro http://example.test
fail_cmd 1 "the introduction point has to be https." \
  run_in "$plain" env -u OVERTO_PASS "$BIN" here --intro http://localhost:9

fail_cmd 1 "pass the address with --direct." \
  env -u OVERTO_INTRO "$BIN" to --direct
fail_cmd 1 "unknown argument: --replace" \
  env -u OVERTO_INTRO "$BIN" to --replace
fail_cmd 1 "Pass the full path of the project." \
  env -u OVERTO_INTRO "$BIN" to proj
fail_cmd 1 "Pass the full path of the project." \
  env -u OVERTO_INTRO "$BIN" to ./proj
fail_cmd 1 "Pass the full path of the project." \
  env -u OVERTO_INTRO "$BIN" to ../proj
fail_cmd 1 "Pass the full path of the project." \
  env -u OVERTO_INTRO "$BIN" to "$ID" proj
fail_cmd 1 "Pass the full path of the project." \
  env -u OVERTO_INTRO "$BIN" to "$ID" docs/notes.md
fail_cmd 1 "that id is not valid." \
  env -u OVERTO_INTRO "$BIN" to '7k9qm3hf-short' --direct 127.0.0.1:9
fail_cmd 1 "pass the id on the terminal." \
  env -u OVERTO_INTRO -u OVERTO_ID_FILE "$BIN" to --direct 127.0.0.1:9 </dev/null

fail_cmd 1 "I only send HANDOFF.md and the agent files." \
  env -u OVERTO_INTRO "$BIN" to '7k9qm3if-4rv2-np8c-wxt3-f6dt-aj5m-eyb2qr' /etc/passwd
if grep -F -q "This id is on the command line" "$TMP/cmd-err"; then
  ok "argv id warns about shell history"
else
  bad "argv id warns about shell history"
fi
printf 'nope\n' > "$plain/docs-notes.md"
fail_cmd 1 "I only send HANDOFF.md and the agent files." \
  env -u OVERTO_INTRO "$BIN" to "$ID" "$plain/docs-notes.md"
fail_cmd 1 "I do not send a root-owned folder." \
  env -u OVERTO_INTRO "$BIN" to "$ID" /etc
fail_cmd 1 "I do not send this folder. Enter the project repo." \
  env -u OVERTO_INTRO "$BIN" to "$ID" /HANDOFF.md

home2=$TMP/home2
mkdir -p "$home2"
fail_cmd 1 "I do not wait in this folder. Enter the project repo." \
  sh -c 'cd "$1" && HOME="$1" exec "$2" here --direct' sh "$home2" "$BIN"
fail_cmd 1 "I do not send this folder. Enter the project repo." \
  sh -c 'cd "$1" && HOME="$1" OVERTO_ID_FILE="$2" exec "$3" to --direct 127.0.0.1:9' \
  sh "$home2" "$TMP/idfile" "$BIN"
ln -s "$home2" "$TMP/homelink"
fail_cmd 1 "I do not send this folder. Enter the project repo." \
  env -u OVERTO_INTRO HOME="$home2" OVERTO_ID_FILE="$TMP/idfile" \
  "$BIN" to --direct 127.0.0.1:9 "$TMP/homelink"

linkproj=$TMP/linkproj
mkdir -p "$linkproj"
ln -s /etc/passwd "$linkproj/HANDOFF.md"
fail_cmd 1 "HANDOFF.md is a link. I only send the file itself." \
  run_in "$linkproj" env -u OVERTO_INTRO OVERTO_ID_FILE="$TMP/idfile" \
  "$BIN" to --direct 127.0.0.1:9

big=$TMP/big
mkdir -p "$big"
dd if=/dev/zero of="$big/HANDOFF.md" bs=262145 count=1 >/dev/null 2>&1
fail_cmd 1 "HANDOFF.md is too big." \
  run_in "$big" env -u OVERTO_INTRO OVERTO_ID_FILE="$TMP/idfile" \
  "$BIN" to --direct 127.0.0.1:9

sumdir=$TMP/sumdir
mkdir -p "$sumdir"
dd if=/dev/zero of="$sumdir/HANDOFF.md" bs=200000 count=1 >/dev/null 2>&1
dd if=/dev/zero of="$sumdir/AGENTS.md" bs=200000 count=1 >/dev/null 2>&1
dd if=/dev/zero of="$sumdir/CLAUDE.md" bs=200000 count=1 >/dev/null 2>&1
fail_cmd 1 "the handoff is too big." \
  run_in "$sumdir" env -u OVERTO_INTRO OVERTO_ID_FILE="$TMP/idfile" \
  "$BIN" to --direct 127.0.0.1:9

closed=$TMP/closed
mkdir -p "$closed"
printf 'hidden\n' > "$closed/HANDOFF.md"
chmod 000 "$closed/HANDOFF.md"
fail_cmd 1 "I cannot read $closed/HANDOFF.md" \
  run_in "$closed" env -u OVERTO_INTRO OVERTO_ID_FILE="$TMP/idfile" \
  "$BIN" to --direct 127.0.0.1:9
chmod 600 "$closed/HANDOFF.md" || true

estado=$TMP/estado
mkdir -p "$estado"
printf 'old\n' > "$estado/ESTADO.md"
fail_cmd 1 "this folder still has ESTADO.md. Run overto init to rename it." \
  run_in "$estado" env -u OVERTO_INTRO OVERTO_ID_FILE="$TMP/idfile" \
  "$BIN" to --direct 127.0.0.1:9

empty=$TMP/empty
mkdir -p "$empty"
fail_cmd 1 "no HANDOFF.md here. Enter the project and run: overto init" \
  run_in "$empty" env -u OVERTO_INTRO OVERTO_ID_FILE="$TMP/idfile" \
  "$BIN" to --direct 127.0.0.1:9

secret=$TMP/secret
mkdir -p "$secret"
printf '%s\n' 'sk-abcdefghijklmnopqrst' > "$secret/HANDOFF.md"
reset_box
fail_cmd 1 "nothing was sent." \
  run_in "$secret" env -u OVERTO_INTRO OVERTO_PASS="$FAKE" OVERTO_FAKE=modes OVERTO_FAKE_BOX="$BOX" \
  OVERTO_ID_FILE="$TMP/idfile" "$BIN" to --direct 127.0.0.1:9 <<'EOF'
n
EOF
if [ -e "$BOX/modes" ]; then
  bad "a refused secret started the helper"
else
  ok "a refused secret started no helper"
fi

reset_box
set +e
run_in "$secret" env -u OVERTO_INTRO OVERTO_PASS="$FAKE" OVERTO_FAKE=modes OVERTO_FAKE_BOX="$BOX" \
  OVERTO_ID_FILE="$TMP/idfile" "$BIN" to --direct 127.0.0.1:9 >"$TMP/cmd-out" 2>"$TMP/cmd-err" <<'EOF'
y
EOF
got=$?
set -e
if [ "$got" -eq 2 ] && [ -f "$BOX/modes" ] && grep -F -q "looks like it contains a secret" "$TMP/cmd-err"; then
  ok "a confirmed secret reaches the helper"
else
  bad "a confirmed secret reaches the helper (exit $got)"
  cat "$TMP/cmd-err" >&2 || true
fi

pem=$TMP/pem
mkdir -p "$pem"
printf '%s\n' '-----BEGIN OPENSSH PRIVATE KEY-----' > "$pem/HANDOFF.md"
reset_box
set +e
run_in "$pem" env -u OVERTO_INTRO OVERTO_PASS="$FAKE" OVERTO_FAKE=modes OVERTO_FAKE_BOX="$BOX" \
  OVERTO_ID_FILE="$TMP/idfile" "$BIN" to --send-secrets --direct 127.0.0.1:9 >"$TMP/cmd-out" 2>"$TMP/cmd-err" </dev/null
got=$?
set -e
if [ "$got" -eq 2 ] && [ -f "$BOX/modes" ] && ! grep -F -q "Send it anyway?" "$TMP/cmd-err"; then
  ok "--send-secrets skips the prompt"
else
  bad "--send-secrets skips the prompt (exit $got)"
  cat "$TMP/cmd-err" >&2 || true
fi
if grep -F -q -- "--send-secrets" "$BOX/argv"; then
  ok "helper argv gets --send-secrets"
else
  bad "helper argv gets --send-secrets"
fi

reset_box
set +e
run_in "$plain" env -u OVERTO_INTRO OVERTO_PASS="$FAKE" OVERTO_FAKE=modes OVERTO_FAKE_BOX="$BOX" \
  "$BIN" here --direct --for 0 >"$TMP/here-out" 2>"$TMP/here-err"
got=$?
set -e
id_line=$(grep -F 'This instance:' "$TMP/here-out" || true)
shown=$(printf '%s\n' "$id_line" | awk '{ print $3 }')
compact=$(printf '%s' "$shown" | tr -d '-')
if [ "$got" -eq 2 ] && [ -f "$BOX/modes" ] \
  && [ "$(sed -n '1p' "$BOX/modes")" = 700 ] \
  && [ "$(sed -n '2p' "$BOX/modes")" = 600 ] \
  && printf '%s\n' "$id_line" | grep -E -q 'This instance: [0-9a-hj-kmnp-tv-z]{8}-([0-9a-hj-kmnp-tv-z]{4}-){5}[0-9a-hj-kmnp-tv-z]{6}' \
  && grep -F -q 'It works until you stop this command.' "$TMP/here-out" \
  && ! grep -F -q 'minutes' "$TMP/here-out" \
  && grep -F -q -- '--direct' "$BOX/argv" \
  && ! grep -F -q "$shown" "$BOX/argv" \
  && ! grep -F -q "$compact" "$BOX/argv"; then
  ok "run directory is private and the id stays out of argv"
else
  bad "run directory is private and the id stays out of argv (exit $got)"
  echo "modes: $(cat "$BOX/modes" 2>/dev/null)" >&2
  echo "argv: $(cat "$BOX/argv" 2>/dev/null)" >&2
  echo "id: $id_line" >&2
  cat "$TMP/here-err" >&2 || true
fi

reset_box
set +e
run_in "$plain" env -u OVERTO_INTRO OVERTO_PASS="$FAKE" OVERTO_FAKE=modes OVERTO_FAKE_BOX="$BOX" \
  "$BIN" here --for 0 --intro 'http://127.0.0.1:9' >"$TMP/here-out" 2>"$TMP/here-err"
got=$?
set -e
if [ "$got" -eq 2 ] && grep -F -q -- '--intro' "$BOX/argv" && grep -F -q 'http://127.0.0.1:9' "$BOX/argv"; then
  ok "loopback http introducer is allowed"
else
  bad "loopback http introducer is allowed (exit $got)"
  cat "$TMP/here-err" >&2 || true
fi

reset_box
set +e
run_in "$plain" env -u OVERTO_INTRO OVERTO_PASS="$FAKE" OVERTO_FAKE=modes OVERTO_FAKE_BOX="$BOX" \
  "$BIN" here --for 0 --intro 'http://[::1]:9' >"$TMP/here-out" 2>"$TMP/here-err"
got=$?
set -e
if [ "$got" -eq 2 ] && grep -F -q 'http://[::1]:9' "$BOX/argv"; then
  ok "loopback ipv6 introducer is allowed"
else
  bad "loopback ipv6 introducer is allowed (exit $got)"
  cat "$TMP/here-err" >&2 || true
fi

reset_box
set +e
run_in "$plain" env OVERTO_PASS="$FAKE" OVERTO_FAKE=modes OVERTO_FAKE_BOX="$BOX" \
  OVERTO_INTRO='https://example.test' "$BIN" here --for 0 >"$TMP/here-out" 2>"$TMP/here-err"
got=$?
set -e
if [ "$got" -eq 2 ] && grep -F -q 'https://example.test' "$BOX/argv"; then
  ok "OVERTO_INTRO is passed to the helper"
else
  bad "OVERTO_INTRO is passed to the helper (exit $got)"
  cat "$TMP/here-err" >&2 || true
fi

fail_cmd 1 "this command needs overto-pass, and it is not next to the overto command." \
  run_in "$plain" env -u OVERTO_PASS -u OVERTO_INTRO OVERTO_ID_FILE="$TMP/idfile" \
  "$BIN" to --direct 127.0.0.1:9
if (cd "$plain" && env -u OVERTO_PASS "$BIN" show >/dev/null); then
  ok "show does not need overto-pass"
else
  bad "show does not need overto-pass"
fi

# Two processes, one box, no sockets.
send=$TMP/send
recv=$TMP/recv
mkdir -p "$send/.cursor/rules" "$send/sub" "$recv"
printf 'Ship the lab handoff.\n' > "$send/HANDOFF.md"
printf 'Agents stay.\n' > "$send/AGENTS.md"
printf 'Cursor rule.\n' > "$send/.cursor/rules/handoff.mdc"
printf 'do not send\n' > "$send/NOTES.md"
printf 'nested handoff\n' > "$send/sub/HANDOFF.md"

start_here "$recv" 'y
y
'
run_to "$TMP" 'y
' "$send"
finish_here
if [ "$to_rc" -eq 0 ] && [ "$here_rc" -eq 0 ] \
  && cmp -s "$send/HANDOFF.md" "$recv/HANDOFF.md" \
  && cmp -s "$send/AGENTS.md" "$recv/AGENTS.md" \
  && cmp -s "$send/.cursor/rules/handoff.mdc" "$recv/.cursor/rules/handoff.mdc" \
  && [ ! -e "$recv/NOTES.md" ] && [ ! -e "$recv/sub/HANDOFF.md" ] \
  && [ "$(mode_of "$recv/HANDOFF.md")" = 600 ] \
  && grep -F -q 'Files are in this folder. Commit them with the project when you want them kept.' "$TMP/here-out" \
  && grep -F -q 'These files tell the next AI what to do.' "$TMP/here-err" \
  && grep -F -q 'Take them? [y/N]' "$TMP/here-err" \
  && grep -F -q 'Trying a direct path.' "$TMP/to-err" \
  && grep -F -q 'Check this code with them: 481-229' "$TMP/to-err" \
  && grep -F -q 'Check this code with them: 481-229' "$TMP/here-err" \
  && grep -F -q 'Listening on:' "$TMP/here-err" \
  && grep -F -q '127.0.0.1:9' "$TMP/here-err" \
  && grep -F -q 'Sending:' "$TMP/to-err" \
  && grep -F -q -- '- HANDOFF.md' "$TMP/to-err" \
  && ! grep -F -q 'Ship the lab handoff.' "$TMP/to-out" \
  && ! grep -F -q 'Ship the lab handoff.' "$TMP/here-out" \
  && ! grep -F -q 'This id is on the command line' "$TMP/to-err"; then
  ok "fake peer delivers the share set"
else
  bad "fake peer delivers the share set (to $to_rc here $here_rc)"
  echo "--- to stderr ---" >&2
  cat "$TMP/to-err" >&2 || true
  echo "--- here stderr ---" >&2
  cat "$TMP/here-err" >&2 || true
  echo "--- here stdout ---" >&2
  cat "$TMP/here-out" >&2 || true
fi

# One allowlisted file, named by its full path.
recv1=$TMP/recv1
mkdir -p "$recv1"
start_here "$recv1" 'y
y
'
run_to "$TMP" 'y
' "$send/.cursor/rules/handoff.mdc"
finish_here
if [ "$to_rc" -eq 0 ] && [ "$here_rc" -eq 0 ] \
  && cmp -s "$send/.cursor/rules/handoff.mdc" "$recv1/.cursor/rules/handoff.mdc" \
  && [ ! -e "$recv1/HANDOFF.md" ] \
  && grep -F -q -- '- .cursor/rules/handoff.mdc' "$TMP/to-err" \
  && ! grep -F -q -- '- HANDOFF.md' "$TMP/to-err"; then
  ok "a full path sends that one agent file"
else
  bad "a full path sends that one agent file (to $to_rc here $here_rc)"
  cat "$TMP/to-err" >&2 || true
  cat "$TMP/here-err" >&2 || true
fi

same=$TMP/same-send
samerecv=$TMP/same-recv
mkdir -p "$same" "$samerecv"
printf 'Ship the lab handoff.\n' > "$same/HANDOFF.md"
cp "$same/HANDOFF.md" "$samerecv/HANDOFF.md"
start_here "$samerecv" 'y
'
run_to "$same" 'y
'
finish_here
if [ "$to_rc" -eq 0 ] && [ "$here_rc" -eq 0 ] \
  && grep -F -q 'HANDOFF.md already matches. Left it.' "$TMP/here-err" \
  && ! grep -F -q 'Take them?' "$TMP/here-err" \
  && cmp -s "$same/HANDOFF.md" "$samerecv/HANDOFF.md"; then
  ok "identical bytes are left in place"
else
  bad "identical bytes are left in place (to $to_rc here $here_rc)"
  cat "$TMP/here-err" >&2 || true
fi

replsend=$TMP/repl-send
replrecv=$TMP/repl-recv
mkdir -p "$replsend" "$replrecv"
printf 'new handoff\n' > "$replsend/HANDOFF.md"
printf 'old handoff\n' > "$replrecv/HANDOFF.md"
start_here "$replrecv" 'y
' --replace
run_to "$replsend" 'y
'
finish_here
if [ "$to_rc" -eq 0 ] && [ "$here_rc" -eq 0 ] \
  && cmp -s "$replsend/HANDOFF.md" "$replrecv/HANDOFF.md" \
  && ! grep -F -q 'Take them?' "$TMP/here-err"; then
  ok "--replace writes without the take prompt"
else
  bad "--replace writes without the take prompt (to $to_rc here $here_rc)"
  cat "$TMP/here-err" >&2 || true
  cat "$TMP/to-err" >&2 || true
fi

declsend=$TMP/decl-send
declrecv=$TMP/decl-recv
mkdir -p "$declsend" "$declrecv"
printf 'nope\n' > "$declsend/HANDOFF.md"
start_here "$declrecv" 'y
n
'
run_to "$declsend" 'y
'
finish_here
if [ "$to_rc" -eq 2 ] && [ "$here_rc" -eq 2 ] \
  && [ ! -e "$declrecv/HANDOFF.md" ] \
  && grep -F -q 'they declined. Nothing was sent.' "$TMP/to-err" \
  && grep -F -q 'nothing was written.' "$TMP/here-err"; then
  ok "a no on take writes nothing"
else
  bad "a no on take writes nothing (to $to_rc here $here_rc)"
  cat "$TMP/to-err" >&2 || true
  cat "$TMP/here-err" >&2 || true
fi

linksend=$TMP/link-send
linkrecv=$TMP/link-recv
mkdir -p "$linksend" "$linkrecv"
printf 'incoming\n' > "$linksend/HANDOFF.md"
printf 'keep me\n' > "$linkrecv/real.md"
ln -s real.md "$linkrecv/HANDOFF.md"
start_here "$linkrecv" 'y
'
run_to "$linksend" 'y
'
finish_here
if [ "$to_rc" -eq 2 ] && [ "$here_rc" -eq 2 ] \
  && [ -L "$linkrecv/HANDOFF.md" ] \
  && grep -F -q 'keep me' "$linkrecv/real.md" \
  && grep -F -q 'HANDOFF.md is a link. I will not follow it.' "$TMP/here-err" \
  && grep -F -q 'nothing was written.' "$TMP/here-err"; then
  ok "a destination symlink is left alone"
else
  bad "a destination symlink is left alone (to $to_rc here $here_rc)"
  cat "$TMP/here-err" >&2 || true
  cat "$TMP/to-err" >&2 || true
fi

sas=$TMP/sas
mkdir -p "$sas"
printf 'secret body\n' > "$sas/HANDOFF.md"
reset_box
set +e
run_in "$sas" env -u OVERTO_INTRO -u OVERTO_FAKE OVERTO_PASS="$FAKE" OVERTO_FAKE_BOX="$BOX" \
  OVERTO_ID_FILE="$TMP/idfile" "$BIN" to --direct 127.0.0.1:9 >"$TMP/to-out" 2>"$TMP/to-err" <<'EOF'
n
EOF
got=$?
set -e
if [ "$got" -eq 2 ] && grep -F -q 'they declined. Nothing was sent.' "$TMP/to-err" \
  && ! grep -F -q 'secret body' "$TMP/to-out"; then
  ok "a no on the code sends nothing"
else
  bad "a no on the code sends nothing (exit $got)"
  cat "$TMP/to-err" >&2 || true
fi

# Real helper, loopback only. Skipped when Go is not installed.
if command -v go >/dev/null 2>&1; then
  intro_pid=
  net_here=
  trap 'if [ -n "$intro_pid" ]; then kill "$intro_pid" 2>/dev/null || true; fi; if [ -n "$net_here" ]; then kill "$net_here" 2>/dev/null || true; fi' EXIT
  if (cd "$ROOT" && go build -o "$TMP/overto-pass" ./cmd/overto-pass); then
    ok "overto-pass builds"
  else
    bad "overto-pass builds"
  fi
  if [ -x "$TMP/overto-pass" ]; then
    poll_file() {
      file=$1
      needle=$2
      ticks=0
      while [ "$ticks" -lt 25 ]; do
        if [ -f "$file" ] && grep -F -q "$needle" "$file"; then
          return 0
        fi
        ticks=$((ticks + 1))
        sleep 1
      done
      return 1
    }
    stop_pid() {
      pid=$1
      if [ -n "$pid" ]; then
        kill "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
      fi
    }
    netsend=$TMP/net-send
    netrecv=$TMP/net-recv
    mkdir -p "$netsend" "$netrecv"
    printf 'live handoff\n' > "$netsend/HANDOFF.md"
    printf 'y\ny\n' > "$TMP/net-here-ans"
    printf 'y\n' > "$TMP/net-to-ans"
    (
      cd "$netrecv" || exit 1
      exec env -u OVERTO_INTRO -u OVERTO_ID_FILE -u OVERTO_SKIP_DIRECT \
        OVERTO_PASS="$TMP/overto-pass" \
        "$BIN" here --for 1 --direct 127.0.0.1:0
    ) <"$TMP/net-here-ans" >"$TMP/net-here-out" 2>"$TMP/net-here-err" &
    net_here=$!
    if poll_file "$TMP/net-here-err" 'Listening on:' && poll_file "$TMP/net-here-out" 'This instance:'; then
      addr=$(awk 'seen && /:/ { print; exit } /Listening on:/ { seen = 1 }' "$TMP/net-here-err")
      sed -n 's/^This instance: //p' "$TMP/net-here-out" > "$TMP/net-id"
      set +e
      (
        cd "$netsend" || exit 1
        exec env -u OVERTO_INTRO -u OVERTO_SKIP_DIRECT \
          OVERTO_PASS="$TMP/overto-pass" \
          OVERTO_ID_FILE="$TMP/net-id" \
          "$BIN" to --direct "$addr"
      ) <"$TMP/net-to-ans" >"$TMP/net-to-out" 2>"$TMP/net-to-err"
      net_to_rc=$?
      wait "$net_here"
      net_here_rc=$?
      set -e
      net_here=
      if [ "$net_to_rc" -eq 0 ] && [ "$net_here_rc" -eq 0 ] \
        && cmp -s "$netsend/HANDOFF.md" "$netrecv/HANDOFF.md"; then
        ok "direct loopback moves HANDOFF.md"
      else
        bad "direct loopback moves HANDOFF.md (to $net_to_rc here $net_here_rc addr $addr)"
        echo "--- here ---" >&2
        cat "$TMP/net-here-err" >&2 || true
        echo "--- to ---" >&2
        cat "$TMP/net-to-err" >&2 || true
      fi
    else
      bad "direct receiver did not listen"
      echo "--- here ---" >&2
      cat "$TMP/net-here-out" >&2 || true
      cat "$TMP/net-here-err" >&2 || true
      stop_pid "$net_here"
      net_here=
    fi

    introsend=$TMP/intro-send
    introrecv=$TMP/intro-recv
    mkdir -p "$introsend" "$introrecv"
    printf 'relay handoff\n' > "$introsend/HANDOFF.md"
    printf 'y\ny\n' > "$TMP/intro-here-ans"
    printf 'y\n' > "$TMP/intro-to-ans"
    "$TMP/overto-pass" intro --http 127.0.0.1:0 >"$TMP/intro-out" 2>"$TMP/intro-err" &
    intro_pid=$!
    if poll_file "$TMP/intro-out" 'overto-pass intro http='; then
      http=$(awk '{ for (i = 1; i <= NF; i++) if (index($i, "http=") == 1) print substr($i, 6) }' "$TMP/intro-out")
      (
        cd "$introrecv" || exit 1
        exec env -u OVERTO_ID_FILE OVERTO_SKIP_DIRECT=1 \
          OVERTO_PASS="$TMP/overto-pass" \
          "$BIN" here --for 1 --intro "http://$http"
      ) <"$TMP/intro-here-ans" >"$TMP/intro-here-out" 2>"$TMP/intro-here-err" &
      net_here=$!
      health_ok=0
      hticks=0
      while [ "$hticks" -lt 20 ]; do
        body=
        if command -v curl >/dev/null 2>&1; then
          body=$(curl -fsS "http://$http/health" 2>/dev/null || true)
        fi
        case $body in
          *'"count":'*)
            case $body in
              *'"count":0'*) ;;
              *) health_ok=1; break ;;
            esac
            ;;
        esac
        hticks=$((hticks + 1))
        sleep 1
      done
      if [ "$health_ok" -eq 1 ] && poll_file "$TMP/intro-here-out" 'This instance:'; then
        sed -n 's/^This instance: //p' "$TMP/intro-here-out" > "$TMP/intro-id"
        set +e
        (
          cd "$introsend" || exit 1
          exec env OVERTO_SKIP_DIRECT=1 \
            OVERTO_PASS="$TMP/overto-pass" \
            OVERTO_ID_FILE="$TMP/intro-id" \
            "$BIN" to --intro "http://$http"
        ) <"$TMP/intro-to-ans" >"$TMP/intro-to-out" 2>"$TMP/intro-to-err"
        intro_to_rc=$?
        wait "$net_here"
        intro_here_rc=$?
        set -e
        net_here=
        if [ "$intro_to_rc" -eq 0 ] && [ "$intro_here_rc" -eq 0 ] \
          && cmp -s "$introsend/HANDOFF.md" "$introrecv/HANDOFF.md" \
          && grep -F -q 'Using the relay.' "$TMP/intro-to-err"; then
          ok "intro relay moves HANDOFF.md"
        else
          bad "intro relay moves HANDOFF.md (to $intro_to_rc here $intro_here_rc)"
          echo "--- intro here ---" >&2
          cat "$TMP/intro-here-err" >&2 || true
          echo "--- intro to ---" >&2
          cat "$TMP/intro-to-err" >&2 || true
        fi
      else
        bad "intro receiver did not register"
        echo "--- intro ---" >&2
        cat "$TMP/intro-out" >&2 || true
        cat "$TMP/intro-here-err" >&2 || true
        stop_pid "$net_here"
        net_here=
      fi
    else
      bad "intro did not start"
      cat "$TMP/intro-err" >&2 || true
    fi
    stop_pid "$intro_pid"
    intro_pid=
  fi
else
  ok "go is absent, network checks skipped"
fi

if [ "$fail" -ne 0 ]; then
  echo "Some tests failed." >&2
  exit 1
fi

echo "All tests passed."
