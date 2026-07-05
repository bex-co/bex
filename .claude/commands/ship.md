---
description: Pull latest main, commit pending changes, and push to origin/main
argument-hint: [optional commit message context...]
allowed-tools: Bash(git status:*), Bash(git pull:*), Bash(git fetch:*), Bash(git diff:*), Bash(git log:*), Bash(git add:*), Bash(git commit:*), Bash(git push:*), Bash(git branch:*)
---

# Task: Ship the current main branch

Bring the local `main` up to date, commit any pending work, and push to `origin/main`.

## Session-aware mode

If you (the agent) made the pending changes yourself earlier in this conversation, you already know what changed and why — do **not** re-derive it from git:

- Skip the `git diff --stat` / `git diff --cached --stat` calls in Step 2. Run only `git status` as a sanity check.
- In Step 4, stage exactly the files you edited this session (by name) and write the commit message from your session knowledge.
- If `git status` shows modified/untracked files you did **not** touch this session, fall back to full inspection for those files (or ask the user) before staging anything beyond your own edits.

Only when the working tree contains changes you didn't make (fresh session, external edits) do the full Step 2 inspection.

## Step 1 — Verify branch

```bash
!git branch --show-current
```

If not on `main`, **STOP** and ask the user whether to switch or abort. Do not silently switch branches.

## Step 2 — Inspect state

```bash
!git status
```

**Session-aware:** if every change listed by `git status` is one you made this session, stop here — no diff commands needed.

Otherwise, inspect the unfamiliar changes:

```bash
!git diff --stat
```

```bash
!git diff --cached --stat
```

If working tree is clean and there's nothing to commit, skip to Step 4 (pull + push).

## Step 3 — Pull latest with rebase

```bash
git pull --rebase origin main
```

If the rebase has conflicts, **STOP** and report. Do not abort the rebase or use `--strategy=ours` without user confirmation.

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

If the push is rejected (non-fast-forward), re-run Step 3 to rebase against the latest, then retry push. Do not force-push to `main`.

## Step 6 — Report

Print one line: current `HEAD` SHA + commit subject, e.g.

```
Shipped: a1b2c3d feat: add tp-backend IDL + CLI
```

## Safety rules

- Never `git push --force` to `main`.
- Never `--no-verify` or skip hooks.
- Never `git reset --hard` or `git checkout .` without user confirmation.
- If anything ambiguous comes up (untracked files, divergent branch, unexpected remote state), stop and ask.

## Optional User Context

$ARGUMENTS
