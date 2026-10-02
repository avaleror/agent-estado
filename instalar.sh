#!/bin/sh
# Deja el comando `estado` disponible en esta máquina.
set -eu

ROOT=$(cd "$(dirname "$0")" && pwd)
DEST="${HOME}/.local/bin"
mkdir -p "$DEST"
ln -sfn "$ROOT/bin/estado" "$DEST/estado"

echo "Listo: $DEST/estado"
echo "Apunta a $ROOT/bin/estado"
echo ""
echo "En cada proyecto:"
echo "  cd carpeta-del-proyecto"
echo "  estado init"

case ":$PATH:" in
  *":$DEST:"*) ;;
  *)
    echo ""
    echo "Añade $DEST a tu PATH para poder ejecutar estado desde cualquier carpeta."
    ;;
esac
