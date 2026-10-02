# estado

Archivos cortos, dentro del propio repositorio, para que un compañero, otra máquina u otra IA sepan en qué está el proyecto y sigan desde ahí.

El estado viaja con `git pull`. Las instrucciones que lee la IA están en inglés, igual que un `CLAUDE.md` de proyecto. El texto de `ESTADO.md` puede ir en español.

## Instalarlo una vez

```sh
git clone https://github.com/avaleror/agent-estado.git ~/GitHub/agent-estado
~/GitHub/agent-estado/instalar.sh
```

Eso deja el comando `estado` en `~/.local/bin`, enlazado a ese clon. Si mueves la carpeta, vuelve a ejecutar `instalar.sh`.

## Usarlo en un proyecto

```sh
cd carpeta-del-proyecto
estado init
```

Aparecen cuatro archivos:

| Archivo | Quién lo lee | Qué contiene |
|---|---|---|
| `AGENTS.md` | Cursor, Codex, Grok, Copilot y Claude | Cómo se trabaja, y la orden de leer y actualizar el estado |
| `ESTADO.md` | La persona y la IA | Goal, Done, Now, Next y Decisions. Aquí va lo que cambia |
| `CLAUDE.md` | Claude | Un puente de pocas líneas hacia `@AGENTS.md` |
| `.cursor/rules/estado.mdc` | Cursor | La misma orden, aplicada siempre |

Revisa la sección **Project** de `AGENTS.md` y rellena `ESTADO.md`. Incluye esos archivos en el commit.

`estado init` se puede volver a ejecutar. Conserva tu texto en `AGENTS.md` y en `CLAUDE.md`, y actualiza el bloque del protocolo, el que va entre las marcas HTML. Si `ESTADO.md` ya existe, lo deja como está.

La regla `.cursor/rules/estado.mdc` se vuelve a escribir. El resto de reglas de esa carpeta se queda.

Quien clone el proyecto ya tiene los archivos. El comando prepara un proyecto que todavía no los tiene. Sirve igual en Cursor, Claude o Grok, y en otra máquina.

## Claude

`CLAUDE.md` se queda corto y estable. Claude lo lee entero al empezar, y a partir de unas 80 líneas las reglas se diluyen. Lo que cambia de una sesión a otra va en `ESTADO.md`.

El puente dice `Follow @AGENTS.md`. Claude abre ese archivo al ver la referencia, así que las reglas viven en un solo sitio.

`.claude.local.md` es para notas de una sola máquina. `estado init` lo añade a `.gitignore`.

El tono personal, las palabras que no quieres ver y la pausa antes de borrar o publicar se quedan en tu configuración de Claude. El kit no los copia a cada proyecto.

## Cada día

Al empezar, la IA tiene que leer `ESTADO.md` y trabajar lo que dice **Next**. Al terminar, tiene que actualizar la fecha, Done, Now, Next y, si hubo una elección que recordar, Decisions.

Si una herramienta no lo hace sola, esta frase basta:

```text
Lee ESTADO.md, sigue AGENTS.md y, al terminar, actualiza ESTADO.md.
```

Para verlo o poner la fecha de hoy:

```sh
estado ver
estado fecha
```

`ESTADO.md` se queda corto. Un ejemplo:

```markdown
# State

Updated: 2026-10-02
Who: Ada

## Goal

Dejar la demo del viernes reproducible en un portátil limpio.

## Done

- El laboratorio levanta.
- Falta el apartado de redes.

## Now

- Página de redes.

## Next

- Escribir el apartado de redes y probarlo con el laboratorio recién creado.

## Decisions

- 2026-10-02: un solo laboratorio. El aula no tiene máquina para dos.

## Blockers

- ninguno
```

Ahí no van contraseñas, tokens ni secretos. El detalle largo se queda en el código y en los commits.

## Para qué es cada pieza

- **Compartir con el equipo.** El estado está en el repo. Quien haga `git pull` ve el mismo objetivo, lo ya hecho y el paso que toca.
- **Gastar pocos tokens.** Son dos archivos breves. La IA lee el siguiente paso y los ficheros que hagan falta para ese paso, y deja el resto del repo sin volcar.
- **Seguir en otra máquina o con otra IA.** El mismo `git pull`. La sesión de terminal que tengas abierta sigue siendo otra cosa: estos archivos son el traspaso.

## Cuando un proyecto crezca

El kit se queda en estos archivos. Hay herramientas aparte para otros problemas:

| Situación | Herramienta |
|---|---|
| Muchas tareas abiertas a la vez | [Beads](https://github.com/gastownhall/beads) |
| Un repo grande y la IA se pierde entre archivos | [Serena](https://github.com/oraios/serena) |
| Hay que enseñarle un repo desconocido a una IA, una sola vez | [Repomix](https://github.com/yamadashy/repomix) o [Gitingest](https://github.com/coderamp-labs/gitingest) |
| Las instrucciones de Claude, Cursor y Codex se han separado | [Rulesync](https://github.com/dyoshikawa/rulesync) |

## Desarrollo de este repo

```sh
sh tests/probar.sh
```

El texto que viaja a los proyectos se edita en `plantillas/protocolo.md` y en `plantillas/claude-puente.md`. Después:

```sh
sh tests/armar.sh
```

Eso reescribe `AGENTS.md` y `CLAUDE.md` para que incluyan ese texto. Las reglas propias de un proyecto van fuera de esas marcas.

Licencia MIT.
