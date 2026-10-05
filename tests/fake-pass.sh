#!/bin/sh
# Test double for overto-pass. It speaks the run-directory contract and
# copies the stage through OVERTO_FAKE_BOX. It does not open a socket.
set -eu

RUN=${OVERTO_RUN:?}
ROLE=${OVERTO_ROLE:?}
BOX=${OVERTO_FAKE_BOX:-}

write_ev() {
  name=$1
  body=$2
  tmp="$RUN/event/.part-$$"
  printf '%s\n' "$body" > "$tmp"
  chmod 600 "$tmp"
  mv "$tmp" "$RUN/event/$name"
}

wait_box() {
  name=$1
  ticks=0
  while [ ! -e "$BOX/$name" ]; do
    ticks=$((ticks + 1))
    if [ "$ticks" -gt 20 ]; then
      write_ev result "error 3 could not reach that instance."
      exit 0
    fi
    sleep 1
  done
}

publish_decision() {
  ans=$1
  tmp="$BOX/.decision-$$"
  printf '%s\n' "$ans" > "$tmp"
  mv "$tmp" "$BOX/decision"
}

if [ "${OVERTO_FAKE:-}" = modes ]; then
  mode_of() {
    case $(uname -s) in
      Darwin) stat -f %Lp "$1" ;;
      *) stat -c %a "$1" ;;
    esac
  }
  mkdir -p "$BOX"
  {
    mode_of "$RUN"
    mode_of "$RUN/id"
  } > "$BOX/modes"
  # Record the helper argv. The instance id must not be one of these.
  : > "$BOX/argv"
  for arg in "$@"; do
    printf '%s\n' "$arg" >> "$BOX/argv"
  done
  exit 0
fi

if [ "$ROLE" = to ]; then
  write_ev path trying
  write_ev sas 481229
  ticks=0
  while [ ! -f "$RUN/cmd/sas" ]; do
    ticks=$((ticks + 1))
    [ "$ticks" -lt 20 ] || exit 1
    sleep 1
  done
  if [ "$(cat "$RUN/cmd/sas")" != yes ]; then
    write_ev result declined
    exit 0
  fi
  [ -n "$BOX" ] || exit 1
  wait_box ready
  mkdir -p "$BOX/files"
  : > "$BOX/manifest.body"
  find "$RUN/stage" -type f > "$BOX/filelist"
  while read -r src; do
    [ -n "$src" ] || continue
    rel=${src#"$RUN/stage/"}
    dest="$BOX/files/$rel"
    mkdir -p "$(dirname "$dest")"
    cp "$src" "$dest"
    if command -v sha256sum >/dev/null 2>&1; then
      sum=$(sha256sum "$src" | awk '{ print $1 }')
    else
      sum=$(shasum -a 256 "$src" | awk '{ print $1 }')
    fi
    case $(uname -s) in
      Darwin) sz=$(stat -f %z "$src") ;;
      *) sz=$(stat -c %s "$src") ;;
    esac
    printf '%s %s %s\n' "$rel" "$sz" "$sum" >> "$BOX/manifest.body"
  done < "$BOX/filelist"
  {
    echo set
    cat "$BOX/manifest.body"
  } > "$BOX/manifest"
  wait_box decision
  if [ "$(cat "$BOX/decision")" != yes ]; then
    write_ev result declined
    exit 0
  fi
  write_ev result done
  exit 0
fi

[ -n "$BOX" ] || exit 1
mkdir -p "$BOX"
write_ev bound "127.0.0.1:9"
write_ev path direct
write_ev sas 481229
: > "$BOX/ready"
ticks=0
while [ ! -f "$RUN/cmd/sas" ]; do
  ticks=$((ticks + 1))
  [ "$ticks" -lt 20 ] || exit 1
  sleep 1
done
if [ "$(cat "$RUN/cmd/sas")" != yes ]; then
  write_ev result declined
  exit 0
fi
wait_box manifest
mkdir -p "$RUN/stage"
while read -r rel sz sum; do
  [ -n "$rel" ] || continue
  src="$BOX/files/$rel"
  dest="$RUN/stage/$rel"
  mkdir -p "$(dirname "$dest")"
  cp "$src" "$dest"
  chmod 600 "$dest"
done < "$BOX/manifest.body"
{
  echo set
  cat "$BOX/manifest.body"
} > "$RUN/event/.manifest-part"
chmod 600 "$RUN/event/.manifest-part"
mv "$RUN/event/.manifest-part" "$RUN/event/manifest"
: > "$RUN/event/staged"
chmod 600 "$RUN/event/staged"
ticks=0
while [ ! -f "$RUN/cmd/files" ]; do
  ticks=$((ticks + 1))
  [ "$ticks" -lt 20 ] || exit 1
  sleep 1
done
answer=$(cat "$RUN/cmd/files")
publish_decision "$answer"
if [ "$answer" != yes ]; then
  write_ev result declined
  exit 0
fi
write_ev result done
exit 0
