# Instrucciones para agentes

<!-- estado-protocolo -->
## Shared state

At the start of a session:

1. Read ESTADO.md.
2. Work on what Next says. If Next is empty, ask before opening another front.

Before you finish:

1. Update ESTADO.md: the date, Done, Now, Next, and Decisions when a choice is worth keeping.
2. Keep ESTADO.md short. Detail lives in the code and in commits.
3. Do not put secrets, passwords, or tokens in it.

To spend few tokens:

- Do not dump the repository or the chat.
- Read only the files the current step needs.
- Do not re-read a file already read in this session unless it changed or you were asked.
- Do not search the tree on a hunch.
- Skip the end-of-task recap. The diff and ESTADO.md are the record.

Keep AGENTS.md and CLAUDE.md stable and short. What changes between sessions goes in ESTADO.md.

Project rules go outside this block. `estado init` replaces the text between these marks.
<!-- /estado-protocolo -->

## Project

Fill this once. Change it only when the way of working changes.

- What this is:
- Why it exists:
- How to build:
- How to test:
- Conventions:
