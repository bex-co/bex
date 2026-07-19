---
description: Pull latest main, commit pending changes, push to origin/main; for substantial code changes, watch CI + the production deploy and fix until the ship lands green
argument-hint: [optional commit message context...]
allowed-tools: Bash(git status:*), Bash(git pull:*), Bash(git fetch:*), Bash(git diff:*), Bash(git log:*), Bash(git add:*), Bash(git commit:*), Bash(git push:*), Bash(git branch:*), Bash(git rev-parse:*), Bash(gh run list:*), Bash(gh run watch:*), Bash(gh run view:*), Bash(gh run rerun:*)
---

# Task: Ship the current main branch

Bring the local `main` up to date, commit any pending work, and push to `origin/main`. Then, **if the push contains substantial code changes** — anything that reaches production (see the Step 6 gate) — **stay on duty until it has actually landed**: watch every CI run the push triggers (the `deploy (bex via Argo)` workflow is the one that reaches prod), and if anything fails, diagnose, fix, re-ship, and keep monitoring until everything is green. For such a ship, `git push` is only the halfway mark — it is done when CI and the prod deploy have succeeded for the shipped HEAD. For trivial pushes (docs, `.pm/**`, `.claude/**`, formatting), a quick CI snapshot and the report are enough.

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

If the working tree is clean and there's nothing to commit, skip Step 4: still do Step 3 (pull), Step 5 (push), and the Step 6 gate — if HEAD carries substantial code changes whose deploy run isn't green yet, that still gets monitored.

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

## Step 6 — Gate: does this ship need monitoring?

**The monitor-and-fix mandate applies only to substantial code changes — the ones that reach production.** A push touching `lego/**`, `dashboard/**`, `deploy/gitops/**`, or `.github/workflows/deploy.yml` triggers the **`deploy (bex via Argo)`** workflow — test gates → image build/sign/CVE-scan → GitOps digest pin (`[skip ci]` write-back commit) → Argo sync → `rollout status` checks against the prod cluster. A green run of that workflow **is** the production deploy.

Decide the tier from the paths in the commits you pushed (in session-aware mode you already know them; the run list below confirms — GitHub has already evaluated the path filters):

```bash
SHA=$(git rev-parse HEAD)
gh run list --commit "$SHA" --json databaseId,workflowName,status,conclusion,url
```

(Runs can take ~30s to register — retry the list a couple of times if empty.)

- **Substantial** — the `deploy (bex via Argo)` run appears (deploy-triggering paths were touched): full monitoring is mandatory. Continue with the watch below and Step 7.
- **Trivial** — only light CI appears (docs prettier, scripts tests, lint, …) or nothing at all: no monitor-and-fix obligation. Take the snapshot above; for fast runs (seconds-to-minutes) one quick confirmation check is courteous, but do not sit in a watch loop, and go straight to Step 8. If nothing was triggered, the ship is good immediately — say so.

### Watch (substantial ships only)

1. Wait for **every** listed run to complete. Prefer `gh run watch <id> --exit-status --interval 30` per run (watch the `deploy (bex via Argo)` run first — it's the long pole, typically 10–20 min). If a shell call hits its timeout while a run is still in progress, just re-invoke the watch (or re-list) — never abandon monitoring because it's taking long. If a `Monitor`-style waiting facility is available in the harness, use it instead of long blocking calls.

2. Success = every triggered run concluded `success` (skipped jobs are fine). Only then move to Step 8. Note: the deploy workflow's write-back commit (`chore(deploy): pin … [skip ci]`) will appear on `origin/main` after a green deploy — that's expected; it triggers no CI and needs no action beyond a `git pull --rebase` next time.

## Step 7 — Fix until green (the monitor-and-fix loop, substantial ships)

If any run fails during a monitored (substantial) ship, do **not** stop and report the failure — fixing it is part of this ship. Commits and pushes made to repair a failing ship are covered by the user's `/ship` invocation; no fresh authorization is needed.

For each failed run:

1. **Diagnose from the logs first:** `gh run view <id> --log-failed` (add `--job <job-id>` to narrow). Read the actual error, not just the step name.
2. **Classify and act:**
   - **Real regression** (test failure in `test-backend`/`test-operator`/`test-dashboard`, lint, build error, CRITICAL CVE gate): reproduce locally where cheap (`cd lego/backend && go test ./...`, `make test` / `make lint-fix` from `lego/operator/`, `yarn test` in `dashboard/`), fix the code, then **re-run Steps 3–5** to commit and push the fix. The monitored SHA becomes the new HEAD; go back to Step 6.
   - **Transient/infra flake** (GHCR or network 5xx, Hetzner API/SSH timeout fetching the kubeconfig, registry blips, the kubelet-TLS caveats called out in `deploy.yml` comments): `gh run rerun <id> --failed`. Give a flake at most 2 reruns before treating it as real.
   - **Cluster-side deploy failure** (Argo app degraded, `rollout status` timeout, CrashLoopBackOff on the new image): the run logs usually include the Argo/pod diagnosis the workflow prints on failure. If more is needed, fetch the prod kubeconfig (`bash scripts/fetch-app-kubeconfig.sh` — needs the `bex` SSH key + Hetzner token from `.env`) and inspect pods/logs directly, then fix (code fix → re-ship, or cluster repair → rerun the failed run). Never `git commit` cluster credentials or `.env` while doing this.
3. **Loop:** after every fix push or rerun, return to Step 6 and watch the new/rerun runs to completion. Repeat until all runs for the current HEAD are green.
4. **Escalate only when genuinely blocked:** the same step has failed 3 consecutive attempts with no inferable fix, or the repair needs credentials/decisions only the user has (e.g. expired out-of-band secrets, a destructive cluster action). Then stop, present the failing run URL, the exact error, what was tried, and one narrow question — never a generic "what should I do?".

## Step 8 — Report

Print one line per outcome: the shipped `HEAD` SHA + subject, plus the deploy/CI verdict, e.g.

```
Shipped: a1b2c3d feat: add tp-backend IDL + CLI — deploy (bex via Argo) ✅ green, prod rolled (run <url>)
```

or, for a trivial ship (no deploy path touched, no monitoring owed):

```
Shipped: a1b2c3d docs: fix ADR link — trivial; docs (prettier) triggered, not monitored
```

If fixes were needed, add one line per fix commit (SHA + what it repaired).

## Safety rules

- Never `git push --force` to `main`.
- Never `--no-verify` or skip hooks.
- Never `git reset --hard` or `git checkout .` without user confirmation.
- Investigate ambiguity using repository state and history before escalating. For untracked files, divergent history, or unexpected remote state, continue when the safe intent is evident; otherwise stop before destructive action and ask one narrow, evidence-backed question.

## Optional User Context

$ARGUMENTS
