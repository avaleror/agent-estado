# Agent instructions

<!-- estado-protocolo -->
## Shared handoff

At the start of a session:

1. Read HANDOFF.md.
2. Work on what Next says. If Next is empty, ask before starting something else.
3. Do not retry an approach listed under Failed.

When an attempt fails:

1. Add one line under Failed: what you tried, and why it failed.
2. Keep the five newest lines. Drop the older ones.
3. Try a different approach, or move the idea to Dropped.

Before you finish:

1. Update HANDOFF.md: the date, Done, Now, Next, Files, and Decisions when a choice is worth keeping. If Files, Failed, or Dropped is missing, add that section.
2. Keep HANDOFF.md short. The detail lives in the code and in the commits.
3. Do not put secrets, passwords, or tokens in it.
4. Do not create a second handoff file. HANDOFF.md is the handoff.

To spend few tokens:

- Do not dump the repository or the chat.
- Read only the files this step needs. Files lists them.
- Do not re-read a file already read in this session unless it changed or you were asked.
- Do not search the tree on a guess.
- Skip the end-of-task recap. The diff and HANDOFF.md are the record.

Keep AGENTS.md and CLAUDE.md stable and short. What changes between sessions goes in HANDOFF.md.

Project rules go outside this block. `overto init` replaces the text between these marks.
<!-- /estado-protocolo -->

## Project

Fill this once. Change it only when the way of working changes.

- What this is:
- Why it exists:
- How to build:
- How to test:
- Conventions:
