# State

Updated: 2026-10-02
Who: Andrés

## Goal

A command that shares a project's current state with teammates and with any AI.

## Done

- Templates for AGENTS.md, ESTADO.md, CLAUDE.md, and the Cursor rule.
- `estado init`, `estado ver`, and `estado fecha`.
- Tests in `tests/probar.sh`.
- Agent instructions aligned with the Claude setup: English, short CLAUDE.md, changing facts in ESTADO.md, `.claude.local.md` gitignored.

## Now

- Repo published and the command installed on this machine.

## Next

- Run `estado init` inside a real project.

## Decisions

- 2026-10-02: the protocol lives in `plantillas/protocolo.md`. `estado init` replaces only that block.
- 2026-10-02: an existing ESTADO.md is left untouched.
- 2026-10-02: CLAUDE.md is a short bridge to AGENTS.md, because Claude follows that file when it exists.
- 2026-10-02: personal tone, banned words, and the pause before deleting or publishing stay in the user's Claude config. The template does not copy them.
- 2026-10-02: Beads, Serena, and full-repo dumps stay out of the kit until a project needs them.

## Blockers

- none
