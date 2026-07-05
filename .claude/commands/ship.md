---
description: Pull latest main, commit pending changes, and push to origin/main
argument-hint: [optional commit message context...]
allowed-tools: Bash(git status:*), Bash(git pull:*), Bash(git fetch:*), Bash(git diff:*), Bash(git log:*), Bash(git add:*), Bash(git commit:*), Bash(git push:*), Bash(git branch:*)
---

# Task: Ship the current main branch

Bring the local `main` up to date, commit any pending work, and push to `origin/main`.

## Step 1 — Verify branch

```bash
!git branch --show-current
```

If not on `main`, **STOP** and ask the user whether to switch or abort. Do not silently switch branches.

## Step 2 — Inspect state

```bash
!git status
```

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

Generate a Conventional Commits message from the diff. Honor `$ARGUMENTS` as additional context if supplied.

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
