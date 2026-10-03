# Instrucciones para agentes

<!-- estado-protocolo -->
## Shared state

At the start of a session:

1. Read ESTADO.md.
2. Work on what Next says. If Next is empty, ask before opening another front.
3. Do not retry an approach listed under Failed.

When an attempt fails:

1. Add one line under Failed: what you tried, and why it failed.
2. Keep the five newest lines. Delete the older ones.
3. Try a different approach, or move the idea to Dropped.

Before you finish:

1. Update ESTADO.md: the date, Done, Now, Next, Files, and Decisions when a choice is worth keeping. If Files, Failed, or Dropped is missing, add that section.
2. Keep ESTADO.md short. Detail lives in the code and in commits.
3. Do not put secrets, passwords, or tokens in it.
4. Do not create a second handoff file. ESTADO.md is the handoff.

To spend few tokens:

- Do not dump the repository or the chat.
- Read only the files the current step needs. Files lists them.
- Do not re-read a file already read in this session unless it changed or you were asked.
- Do not search the tree on a hunch.
- Skip the end-of-task recap. The diff and ESTADO.md are the record.

Keep AGENTS.md and CLAUDE.md stable and short. What changes between sessions goes in ESTADO.md.

Project rules go outside this block. `estado init` replaces the text between these marks.
<!-- /estado-protocolo -->

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
