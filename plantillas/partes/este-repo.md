## This repository

What this is: the `estado` command and the templates that keep a project's current state inside the repo, so a teammate, another machine, or another AI can continue the work.

Why it exists: shared state without pasting a chat or dumping the tree.

How to test: `sh tests/probar.sh`

Conventions:

- Edit the protocol only in `plantillas/protocolo.md`.
- Edit the Claude bridge only in `plantillas/claude-puente.md`.
- After either edit, run `sh tests/armar.sh` so the files that include them are rewritten.
- `plantillas/AGENTS.md` and `AGENTS.md` include the protocol. `estado init` replaces that block.
- Project-specific rules stay outside the block. `estado init` leaves them in place.
- Agent instructions are in English, short, and stable. What changes between sessions goes in ESTADO.md.
- Do not use an em dash in files this repo writes.
- Personal tone and vocabulary stay in the user's own Claude config. Do not copy them into the template.
- Keep CLAUDE.md under a short bridge. A long CLAUDE.md dilutes its own rules.
