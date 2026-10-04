# Handoff

Updated: 2026-10-04
Who: Andrés

## Goal

A command that shares a project's current handoff with teammates and with any AI.

## Done

- Templates for AGENTS.md, HANDOFF.md, CLAUDE.md, and the Cursor rule.
- `overto init`, `overto show`, `overto date`, and `overto share`.
- Tests in `tests/test.sh`.
- The repo text is English: short, direct, and written for people as well as agents.
- The handoff file is `HANDOFF.md`. `overto init` renames an old `ESTADO.md` when the new file is missing.
- The command is `overto`. It is the phrase you say when you pass the work to a person or to an AI.

## Now

- The kit is published and the command is installed on this machine.

## Next

- Run `overto init` inside a real project so the protocol block and the filename update. Then `overto share` to send that project to a coworker.

## Files

- templates/protocol.md
- templates/HANDOFF.md

## Failed

- none

## Dropped

- A resident memory database. It would not travel with git or with Obsidian Sync.

## Decisions

- 2026-10-02: the protocol lives in `templates/protocol.md`. `overto init` replaces only that block.
- 2026-10-02: an existing handoff file is left untouched. On 2026-10-04 the filename became `HANDOFF.md`. init renames `ESTADO.md` when `HANDOFF.md` is missing.
- 2026-10-02: CLAUDE.md is a short bridge to AGENTS.md, because Claude follows that file when it exists.
- 2026-10-02: personal tone, the word list, and the pause before deleting or publishing stay in the user's Claude config. The template does not copy them.
- 2026-10-02: Beads, Serena, and full-repo dumps stay out of the kit until a project needs them.
- 2026-10-03: failed attempts stay in the handoff, five lines at most. There is no second handoff file.
- 2026-10-04: the handoff file is `HANDOFF.md`. The repo text is English. The command is `overto`.

## Blockers

- none
