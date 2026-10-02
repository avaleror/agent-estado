#!/bin/sh
# Pruebas del comando estado. Se ejecutan en directorios temporales.
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
ESTADO="$ROOT/bin/estado"
fail=0

ok() {
  echo "ok: $*"
}

bad() {
  echo "FALLO: $*" >&2
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
    ok "existe $1"
  else
    bad "falta $1"
  fi
}

bloque() {
  file=$1
  start=$2
  end=$3
  awk -v start="$start" -v end="$end" '
    $0 == start { printing = 1 }
    printing { print }
    $0 == end { printing = 0 }
  ' "$file"
}

# Las copias embebidas tienen que coincidir con la fuente.
sh "$ROOT/tests/armar.sh" >/dev/null

if bloque "$ROOT/plantillas/AGENTS.md" '<!-- estado-protocolo -->' '<!-- /estado-protocolo -->' | cmp -s - "$ROOT/plantillas/protocolo.md"; then
  ok "protocolo dentro de plantillas/AGENTS.md"
else
  bad "protocolo dentro de plantillas/AGENTS.md"
fi

if bloque "$ROOT/AGENTS.md" '<!-- estado-protocolo -->' '<!-- /estado-protocolo -->' | cmp -s - "$ROOT/plantillas/protocolo.md"; then
  ok "protocolo dentro de AGENTS.md"
else
  bad "protocolo dentro de AGENTS.md"
fi

if bloque "$ROOT/plantillas/CLAUDE.md" '<!-- estado-claude -->' '<!-- /estado-claude -->' | cmp -s - "$ROOT/plantillas/claude-puente.md"; then
  ok "puente dentro de plantillas/CLAUDE.md"
else
  bad "puente dentro de plantillas/CLAUDE.md"
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# No inicializar la carpeta personal.
home="$TMP/home"
mkdir -p "$home"
if (cd "$home" && HOME="$home" "$ESTADO" init >/dev/null 2>&1); then
  bad "init en HOME debería fallar"
else
  ok "init en HOME rechazado"
fi
if [ -e "$home/AGENTS.md" ]; then
  bad "init en HOME no debería crear archivos"
else
  ok "HOME intacto"
fi

# Proyecto nuevo.
proy="$TMP/proy"
mkdir -p "$proy"
git -C "$proy" init -q -b main
git -C "$proy" config user.name "Ada Prueba"
git -C "$proy" config user.email "ada@example.com"
(cd "$proy" && "$ESTADO" init >/dev/null)

assert_file "$proy/AGENTS.md"
assert_file "$proy/ESTADO.md"
assert_file "$proy/CLAUDE.md"
assert_file "$proy/.cursor/rules/estado.mdc"

hoy=$(date +%Y-%m-%d)
if grep -q "^Updated: $hoy$" "$proy/ESTADO.md"; then
  ok "fecha de hoy"
else
  bad "fecha de hoy"
fi
if grep -q "^Who: Ada Prueba$" "$proy/ESTADO.md"; then
  ok "nombre de git"
else
  bad "nombre de git"
fi
if grep -q -x '.claude.local.md' "$proy/.gitignore"; then
  ok "gitignore local creado"
else
  bad "gitignore local creado"
fi

n_proto=$(grep -c -F '<!-- estado-protocolo -->' "$proy/AGENTS.md" || true)
n_claude=$(grep -c -F '<!-- estado-claude -->' "$proy/CLAUDE.md" || true)
if [ "$n_proto" -eq 1 ] && [ "$n_claude" -eq 1 ]; then
  ok "una sola marca en archivos nuevos"
else
  bad "marcas duplicadas en archivos nuevos ($n_proto, $n_claude)"
fi

# Segunda pasada: no duplica y no pisa ESTADO.md.
printf '\n- hito propio\n' >> "$proy/ESTADO.md"
cp "$proy/AGENTS.md" "$proy/AGENTS.antes"
cp "$proy/ESTADO.md" "$proy/ESTADO.antes"
(cd "$proy" && "$ESTADO" init >/dev/null)
if cmp -s "$proy/AGENTS.antes" "$proy/AGENTS.md"; then
  ok "segundo init no cambia AGENTS.md"
else
  bad "segundo init cambió AGENTS.md"
fi
if cmp -s "$proy/ESTADO.antes" "$proy/ESTADO.md"; then
  ok "segundo init no pisa ESTADO.md"
else
  bad "segundo init pisó ESTADO.md"
fi
n_local=$(grep -c -F '.claude.local.md' "$proy/.gitignore" || true)
if [ "$n_local" -eq 1 ]; then
  ok "gitignore local sin duplicar"
else
  bad "gitignore local duplicado ($n_local)"
fi

# Archivos ya existentes: se conserva el texto y se sustituye el bloque viejo.
viejo="$TMP/viejo"
mkdir -p "$viejo"
cat > "$viejo/AGENTS.md" <<'EOF'
# Reglas del equipo

No borres este párrafo.

<!-- estado-protocolo -->
texto viejo que debe desaparecer
<!-- /estado-protocolo -->

## Proyecto

Esta sección se queda.
EOF
cat > "$viejo/CLAUDE.md" <<'EOF'
# Preferencias

Responde en español.

<!-- estado-claude -->
puente viejo
<!-- /estado-claude -->

## Notas

Esta nota se queda.
EOF
printf '%s\n' '# Estado' 'Actualizado: 1999-01-01' 'HECHO PERSONAL' > "$viejo/ESTADO.md"

(cd "$viejo" && "$ESTADO" init >/dev/null)

if grep -q 'No borres este párrafo.' "$viejo/AGENTS.md" && grep -q 'Esta sección se queda.' "$viejo/AGENTS.md"; then
  ok "texto propio de AGENTS.md conservado"
else
  bad "texto propio de AGENTS.md"
fi
if grep -q 'texto viejo' "$viejo/AGENTS.md"; then
  bad "el protocolo viejo sigue en AGENTS.md"
else
  ok "protocolo viejo sustituido"
fi
if bloque "$viejo/AGENTS.md" '<!-- estado-protocolo -->' '<!-- /estado-protocolo -->' | cmp -s - "$ROOT/plantillas/protocolo.md"; then
  ok "protocolo actualizado"
else
  bad "protocolo actualizado"
fi
if grep -q 'Responde en español.' "$viejo/CLAUDE.md" && grep -q 'Esta nota se queda.' "$viejo/CLAUDE.md"; then
  ok "texto propio de CLAUDE.md conservado"
else
  bad "texto propio de CLAUDE.md"
fi
if grep -q 'puente viejo' "$viejo/CLAUDE.md"; then
  bad "el puente viejo sigue"
else
  ok "puente viejo sustituido"
fi
if grep -q 'HECHO PERSONAL' "$viejo/ESTADO.md" && grep -q '1999-01-01' "$viejo/ESTADO.md"; then
  ok "ESTADO.md existente intacto"
else
  bad "ESTADO.md existente"
fi

# Sin marcas: el bloque se añade una sola vez.
libre="$TMP/libre"
mkdir -p "$libre"
printf '%s\n' '# Solo mio' > "$libre/AGENTS.md"
printf '%s\n' '# Claude mio' > "$libre/CLAUDE.md"
(cd "$libre" && "$ESTADO" init >/dev/null)
(cd "$libre" && "$ESTADO" init >/dev/null)
n_proto=$(grep -c -F '<!-- estado-protocolo -->' "$libre/AGENTS.md" || true)
n_claude=$(grep -c -F '<!-- estado-claude -->' "$libre/CLAUDE.md" || true)
if [ "$n_proto" -eq 1 ] && [ "$n_claude" -eq 1 ] && grep -q 'Solo mio' "$libre/AGENTS.md" && grep -q 'Claude mio' "$libre/CLAUDE.md"; then
  ok "bloque añadido una sola vez"
else
  bad "bloque añadido una sola vez ($n_proto, $n_claude)"
fi

# Marca de inicio sin cierre: no toca nada.
roto="$TMP/roto"
mkdir -p "$roto"
printf '%s\n' '# Roto' '<!-- estado-protocolo -->' 'sin cierre' > "$roto/AGENTS.md"
cp "$roto/AGENTS.md" "$roto/AGENTS.antes"
if (cd "$roto" && "$ESTADO" init >/dev/null 2>&1); then
  bad "init con marca rota debería fallar"
else
  ok "init con marca rota rechazado"
fi
if cmp -s "$roto/AGENTS.antes" "$roto/AGENTS.md" && [ ! -e "$roto/ESTADO.md" ]; then
  ok "marca rota no escribe archivos"
else
  bad "marca rota escribió o cambió archivos"
fi

# ver y fecha.
if (cd "$proy" && "$ESTADO" ver) | grep -q 'hito propio'; then
  ok "ver muestra ESTADO.md"
else
  bad "ver"
fi
(cd "$proy" && "$ESTADO" fecha >/dev/null)
if grep -q "^Updated: $hoy$" "$proy/ESTADO.md" && grep -q 'hito propio' "$proy/ESTADO.md"; then
  ok "fecha cambia el día y conserva el resto"
else
  bad "fecha"
fi

es="$TMP/es"
mkdir -p "$es"
printf '%s\n' '# State' 'Actualizado: 1999-01-01' 'nota' > "$es/ESTADO.md"
(cd "$es" && "$ESTADO" fecha >/dev/null)
if grep -q "^Actualizado: $hoy$" "$es/ESTADO.md" && grep -q '^nota$' "$es/ESTADO.md"; then
  ok "fecha conserva la etiqueta Actualizado"
else
  bad "fecha conserva la etiqueta Actualizado"
fi

ign="$TMP/ign"
mkdir -p "$ign"
printf '*.log' > "$ign/.gitignore"
(cd "$ign" && "$ESTADO" init >/dev/null)
(cd "$ign" && "$ESTADO" init >/dev/null)
n_local=$(grep -c -F '.claude.local.md' "$ign/.gitignore" || true)
if [ "$n_local" -eq 1 ] && grep -q '^\*\.log$' "$ign/.gitignore"; then
  ok "gitignore existente conserva sus reglas"
else
  bad "gitignore existente ($n_local)"
fi

# Orden desconocida.
if "$ESTADO" no-existe >/dev/null 2>&1; then
  bad "orden desconocida debería fallar"
else
  ok "orden desconocida rechazada"
fi

# El comando sigue encontrando las plantillas aunque se invoque por un enlace.
linked="$TMP/bin"
mkdir -p "$linked"
ln -s "$ESTADO" "$linked/estado"
via="$TMP/via"
mkdir -p "$via"
(cd "$via" && PATH="$linked:$PATH" estado init >/dev/null)
assert_file "$via/AGENTS.md"
assert_file "$via/ESTADO.md"

if [ "$fail" -ne 0 ]; then
  echo "Hay pruebas fallidas." >&2
  exit 1
fi

echo "Todas las pruebas pasaron."
