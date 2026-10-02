# Instrucciones para agentes

<!-- estado-protocolo -->
## Estado compartido

Al empezar una sesión:

1. Lee ESTADO.md.
2. Trabaja lo que dice «Siguiente». Si está vacío, pregunta antes de abrir otro frente.

Antes de dar la tarea por terminada:

1. Actualiza ESTADO.md: la fecha, Hecho, Ahora, Siguiente y, si tomaste una decisión que haya que recordar, Decisiones.
2. Déjalo corto. El detalle vive en el código y en los commits.
3. No pongas secretos, contraseñas ni tokens.

Para gastar pocos tokens:

- No vuelques el repositorio ni el historial del chat.
- Lee solo los archivos necesarios para el paso en curso.

Las reglas propias del proyecto van fuera de este bloque. `estado init` sustituye lo que hay entre estas marcas.
<!-- /estado-protocolo -->

## Este repositorio

Qué es: el comando `estado` y las plantillas que dejan el estado de un proyecto dentro del propio repo, para compartirlo con compañeros y seguir en otra máquina o con otra IA.

Cómo se prueba: `sh tests/probar.sh`

Convenciones:

- El protocolo se edita solo en `plantillas/protocolo.md`.
- El puente de Claude se edita solo en `plantillas/claude-puente.md`.
- Después de editarlos, corre `sh tests/armar.sh` para reescribir los archivos que los incluyen.
- `plantillas/AGENTS.md` y `AGENTS.md` incluyen el protocolo. `estado init` sustituye ese bloque.
- Fuera del bloque va lo propio del proyecto. Ahí no entra `estado init`.
- Frases cortas. El objetivo es gastar pocos tokens.
- Al cambiar el protocolo, corre las pruebas.
