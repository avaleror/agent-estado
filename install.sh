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
