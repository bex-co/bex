---
name: ship
description: >-
  Safely bring main up to date, commit intended pending changes, and push to origin/main. Use when the user explicitly asks to ship the current main branch or invokes the repository's ship workflow.


allowed-tools: Bash(git status:*), Bash(git pull:*), Bash(git fetch:*), Bash(git diff:*), Bash(git log:*), Bash(git add:*), Bash(git commit:*), Bash(git push:*), Bash(git branch:*), Bash(git rev-parse:*), Bash(git rebase:*), Bash(git show:*), Bash(git checkout:*), Bash(git reset:*)
---

# Task: Ship the current main branch

Bring the local `main` up to date, commit any pending work, and push to `origin/main`. A successful push is the end of the ship — do not watch CI, do not monitor the deploy, do not chase runs to green. Report the pushed HEAD and stop.

## Session-aware mode

If you (the agent) made the pending changes yourself earlier in this conversation, you already know what changed and why — do **not** re-derive it from git:

- Skip the `git diff --stat` / `git diff --cached --stat` calls in Step 2. Run only `git status` as a sanity check.
- In Step 4, stage exactly the files you edited this session (by name) and write the commit message from your session knowledge.
- If `git status` shows modified/untracked files you did **not** touch this session, fall back to full inspection for those files (or ask the user) before staging anything beyond your own edits.

Only when the working tree contains changes you didn't make (fresh session, external edits) do the full Step 2 inspection.

## Step 1 — Verify branch

```bash
git branch --show-current
```

If not on `main`, **STOP** and ask the user whether to switch or abort. Do not silently switch branches.

## Step 2 — Inspect state

```bash
git status
```

**Session-aware:** if every change listed by `git status` is one you made this session, stop here — no diff commands needed.

Otherwise, inspect the unfamiliar changes:

```bash
git diff --stat
```

```bash
git diff --cached --stat
```

If the working tree is clean and there's nothing to commit, skip Step 4: still do Step 3 (pull) and Step 5 (push).

## Step 3 — Pull latest with rebase

```bash
git pull --rebase origin main
```

If the pull/rebase has conflicts, resolve them proactively and continue shipping. A conflict by itself is not a reason to stop or ask the user what to do.

1. Inspect `git status`, the unmerged-file list (`git diff --name-only --diff-filter=U`), each combined diff, and relevant surrounding code/history. For difficult cases, inspect both index stages (`git show :2:<path>` and `git show :3:<path>`) and the commits being replayed.
2. Infer the intent of both sides and produce the smallest coherent merge that preserves both whenever possible. Follow current repository conventions and update dependent code, tests, generated outputs, or documentation when the combined result requires it.
3. Do not resolve wholesale with `--ours`, `--theirs`, `--strategy=ours`, or by blindly choosing the newer side. Use a side-specific version only when inspection shows that it is the complete intended result for that file.
4. Remove all conflict markers, run the most relevant formatting, generation, and tests that are practical, and review the resolved diff for accidental loss.
5. Stage only the resolved paths explicitly, run `git rebase --continue` (with a non-interactive editor if needed), and repeat until the rebase completes. If an autostash is restored with conflicts after the rebase, resolve and validate those conflicts with the same care, but do not run `git rebase --continue` when no rebase is active.

Exhaust repository evidence and reasonable repairs before escalating. Escalate only when competing resolutions would materially change behavior and the intended choice cannot be inferred safely, or when resolution requires unavailable credentials/external state. Leave the worktree and rebase state intact, explain the exact files and competing semantics, and ask one narrow decision question—never a generic “what should I do?” Do not abort the rebase unless the user directs it.

## Step 4 — Stage and commit (if changes pending)

If there are unstaged changes, stage only the relevant files explicitly. Do **not** use `git add -A` or `git add .` (avoid sweeping in `.env`, secrets, or unrelated files).

Generate a Conventional Commits message — from your session knowledge if you made the changes (session-aware mode), otherwise from the diff. Honor `$ARGUMENTS` as additional context if supplied.

- Briefly describe UI before/after for frontend changes.
- !!Important!! Never mention `Generated with Claude Code` or `Co-Authored-By`.

```bash
git commit -m "$(cat <<'EOF'
<message>
EOF
)"
```

If a pre-commit hook fails, fix the underlying issue and create a NEW commit. Do not use `--no-verify` or `--amend`.

## Step 5 — Push

```bash
git push origin main
```

If the push is rejected (non-fast-forward), re-run Step 3, resolve any conflicts using its procedure, and retry push. Do not force-push to `main`.

## Step 6 — Report

Once the push succeeds, the ship is done. Print one line — the shipped `HEAD` SHA + subject:

```
Shipped: a1b2c3d feat: add tp-backend IDL + CLI
```

Do not watch CI runs, monitor the deploy, or report on them.

## Safety rules

- Never `git push --force` to `main`.
- Never `--no-verify` or skip hooks.
- Never `git reset --hard` or `git checkout .` without user confirmation.
- Investigate ambiguity using repository state and history before escalating. For untracked files, divergent history, or unexpected remote state, continue when the safe intent is evident; otherwise stop before destructive action and ask one narrow, evidence-backed question.

## Optional User Context

$ARGUMENTS
