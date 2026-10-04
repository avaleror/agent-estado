# overto

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="brand/lockup-on-dark.png">
  <img src="brand/lockup.png" alt="overto" width="280">
</picture>

Short files, inside the repo, so a teammate, another machine, or another AI can see where the project is and pick up from there.

The site is [avaleror.github.io/overto](https://avaleror.github.io/overto/).

The name is the line you say when you pass the work on. Over to you. A coworker can take it, and so can an AI.

The handoff travels with `git pull`. The file is `HANDOFF.md`. The instructions an AI reads are in English.

## Install it once

```sh
git clone https://github.com/avaleror/overto.git ~/GitHub/overto
~/GitHub/overto/install.sh
```

That puts the `overto` command in `~/.local/bin`, linked to this clone. If you move the folder, run `install.sh` again.

## Use it in a project

```sh
cd project-folder
overto init
```

Four files show up:

| File | Who reads it | What it holds |
|---|---|---|
| `AGENTS.md` | Cursor, Codex, Grok, Copilot, and Claude | How to work here, and the order to read and update the handoff |
| `HANDOFF.md` | You and the AI | Goal, Done, Now, Next, Files, Failed, and Decisions. This is what changes |
| `CLAUDE.md` | Claude | A few lines that point at `@AGENTS.md` |
| `.cursor/rules/handoff.mdc` | Cursor | The same order, always on |

Check the **Project** section of `AGENTS.md` and fill in `HANDOFF.md`. Commit those files.

You can run `overto init` again. Your text in `AGENTS.md` and `CLAUDE.md` stays, and the protocol block between the HTML marks is refreshed.

If `HANDOFF.md` already exists, it is left as it is. An old `ESTADO.md` is renamed to `HANDOFF.md` when the new file is missing.

The Cursor rule is written again. The other rules in that folder stay. A leftover `.cursor/rules/estado.mdc` from an older run is removed, because that file was always rewritten.

## Claude

`CLAUDE.md` stays short and stable. Claude reads the whole file at the start, and past about 80 lines the rules get thin. What changes from one session to the next goes in `HANDOFF.md`.

The bridge says `Follow @AGENTS.md`. Claude opens that file when it sees the reference, so the rules live in one place.

`.claude.local.md` is for notes that belong on one machine. `overto init` adds it to `.gitignore`.

Your personal tone, the words you do not want, and the pause before deleting or publishing stay in your Claude config. The kit does not copy them into every project.

## Each day

At the start, the AI reads `HANDOFF.md` and works on **Next**. If something fails, it writes one line under **Failed** and does not take that path again. At the end, it updates the date, Done, Now, Next, Files, and Decisions when a choice is worth keeping.

If a tool does not do this on its own, this line is enough:

```text
Read HANDOFF.md, follow AGENTS.md, and update HANDOFF.md before you finish.
```

To print it, or to set today's date:

```sh
overto show
overto date
overto share Ada
```

`show` also answers to `ver`. `date` also answers to `fecha`.

`overto share` prints a note for this one project. It names the repo, the remote when there is one, the next step, and the line your coworker can give their AI. Send that note.

The files travel with the repo. If there is no remote yet, the note lists the files to send instead.

Keep `HANDOFF.md` short. Here is one:

```markdown
# Handoff

Updated: 2026-10-02
Who: Ada

## Goal

Make Friday's demo reproducible on a clean laptop.

## Done

- The lab boots.
- The network section is still missing.

## Now

- Network page.

## Next

- Write the network section and try it on a freshly created lab.

## Files

- docs/network.md

## Failed

- Try the network inside the lab we already used. The old state hides the bug.

## Dropped

- none

## Decisions

- 2026-10-02: one lab. The classroom has no machine for two.

## Blockers

- none
```

Passwords, tokens, and secrets do not go in there. The long detail stays in the code and in the commits.

## What each piece is for

- **Share it with the team.** The handoff is in the repo. A `git pull` shows the same goal, what is already done, and the step that is next.
- **Spend few tokens.** Two short files. The AI reads the next step and the files that step needs, and leaves the rest of the repo alone.
- **Continue on another machine, or with another AI.** Same `git pull`. The terminal session you have open is a different thing. These files are the handoff.

## When a project grows

The kit stays these files. Other tools cover other problems:

| Situation | Tool |
|---|---|
| Many tasks open at once | [Beads](https://github.com/gastownhall/beads) |
| A large repo and the AI gets lost among files | [Serena](https://github.com/oraios/serena) |
| You need to show an unknown repo to an AI, once | [Repomix](https://github.com/yamadashy/repomix) or [Gitingest](https://github.com/coderamp-labs/gitingest) |
| The Claude, Cursor, and Codex instructions have drifted apart | [Rulesync](https://github.com/dyoshikawa/rulesync) |

## Working on this repo

```sh
sh tests/test.sh
```

The text that lands in projects is edited in `templates/protocol.md` and `templates/claude-bridge.md`. Then:

```sh
sh tests/build.sh
```

That rewrites `AGENTS.md` and `CLAUDE.md` so they include that text. A project's own rules go outside those marks.

MIT license.
