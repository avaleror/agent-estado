## This repository

What this is: the `overto` command and the templates that keep a project's current handoff inside the repo, so a teammate, another machine, or another AI can continue the work.

Why it exists: shared handoff without pasting a chat or dumping the tree.

How to test: `sh tests/test.sh`

Conventions:

- Edit the protocol only in `templates/protocol.md`.
- Edit the Claude bridge only in `templates/claude-bridge.md`.
- After either edit, run `sh tests/build.sh` so the files that include them are rewritten.
- `templates/AGENTS.md` and `AGENTS.md` include the protocol. `overto init` replaces that block.
- Project-specific rules stay outside the block. `overto init` leaves them in place.
- Write in English. Short sentences. No em dash.
- What changes between sessions goes in HANDOFF.md.
- Personal tone and the word list stay in the user's own Claude config. Do not copy them into a project.
- Keep CLAUDE.md a short bridge. A long CLAUDE.md makes its own rules easy to miss.
