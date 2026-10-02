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
