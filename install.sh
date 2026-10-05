#!/bin/sh
# Put the `overto` command on this machine.
set -eu

ROOT=$(cd "$(dirname "$0")" && pwd)
DEST="${HOME}/.local/bin"
mkdir -p "$DEST"
ln -sfn "$ROOT/bin/overto" "$DEST/overto"

if [ -L "$DEST/estado" ]; then
  target=$(readlink "$DEST/estado")
  case $target in
    "$ROOT/bin/estado"|"$ROOT/bin/overto")
      rm -f "$DEST/estado"
      echo "Removed the old estado command."
      ;;
  esac
fi

if command -v go >/dev/null 2>&1; then
  if (cd "$ROOT" && go build -o "$ROOT/bin/overto-pass" ./cmd/overto-pass); then
    echo "Built $ROOT/bin/overto-pass"
  else
    echo "overto-pass did not build. init, show, date, and share still work. here and to need the helper."
  fi
else
  echo "Go is not on PATH, so overto-pass was not built. init, show, date, and share still work. here and to need the helper."
fi

echo "Ready: $DEST/overto"
echo "It points at $ROOT/bin/overto"
echo ""
echo "In each project:"
echo "  cd project-folder"
echo "  overto init"

case ":$PATH:" in
  *":$DEST:"*) ;;
  *)
    echo ""
    echo "Add $DEST to your PATH so you can run overto from any folder."
    ;;
esac
