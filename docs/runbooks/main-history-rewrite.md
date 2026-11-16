# Runbook: rewrite `main`'s history on purpose

`main` cannot be deleted or force-pushed: the repository ruleset **`main history`** (id `27183382`, w3/m365) carries `deletion` and `non_fast_forward` rules with no bypass actor. Ordinary fast-forward pushes — every `/ship`, the image-pin bot's `chore(deploy): pin platform images` commits — are unaffected, and no pull request or status check is required.

A rewrite is sometimes the right call. On 2026-11-04 `main` was rewritten twice to purge build artifacts and evidence dumps (AGENTS.md, `.pm/DO_NOT_DO.md` #NO-ARTIFACTS-IN-GIT). This is the only sanctioned path for the next one. Never rewrite `main` to remove a credential that has been rotated: a revoked credential in history is harmless, and rotation is the fix (ADR019 §Decision 6).

## Steps

1. **Decide and announce.** A rewrite needs the user's decision. Every other session sharing a checkout must save its work first.
2. **Disable the ruleset** (an admin token is required):

   ```bash
   gh api -X PUT repos/bex-co/bex/rulesets/27183382 -f enforcement=disabled -q .enforcement
   ```

3. **Rewrite and force-push** (for example `git filter-repo …`, then `git push --force-with-lease origin main`). Release tags (`bex-cli/v*`, `bex-mobile/v*`) are protected by the separate `bex cli release tags` ruleset; leave them alone.
4. **Re-enable the ruleset at once**, even if the push failed:

   ```bash
   gh api -X PUT repos/bex-co/bex/rulesets/27183382 -f enforcement=active -q .enforcement
   ```

5. **Verify:** `bash scripts/github-repo-security-drift.sh` exits 0 and prints `OK:`; `gh api repos/bex-co/bex/rules/branches/main -q '[.[].type]|join(",")'` lists `deletion,non_fast_forward`.
6. **Tell every checkout to reset:** a checkout from before the rewrite must `git fetch && git reset --hard origin/main` after saving its own work, never `git pull --rebase`, which would push the old history back (AGENTS.md).
7. **Purge the old history everywhere else it is public.** A rewrite only changes `main`. Every other branch built on the old history keeps all of it public, and so does every pull request's `refs/pull/<n>/head`, which the repository cannot delete.
   - Delete the stale branches. Check each for unmerged work first: `git cherry origin/main origin/<branch>` lists commits not on `main`.
   - Ask GitHub Support to remove the pull request refs and garbage-collect the unreachable objects ("Removing sensitive data from a repository").

**Residue of the 2026-11-04 rewrites (checked 2026-11-16).**

- Eight branches still held the old history, back to 2026-07-04: `copilot/fix-inconsistent-outdated-docs` and seven `bex-agent/*` branches. They were deleted on 2026-11-16; the tips are kept locally under `refs/backup/2026-11-16/`.
- 44 pull request refs still hold the old history: #37, #39–#52, #54–#59, #78, #79, #81, #83, #85, #88, #90–#92, #94–#96, #102, #104, #106, #110, #114–#118, #121 and #124. Only GitHub Support can remove them.
- Neither holds a credential. A gitleaks scan of all 12,311 of their commits that are not on `main` found the same non-credential shapes as ADR019 §6's full-history scan.

## If the ruleset is missing

Recreate it with the same shape:

```bash
gh api -X POST repos/bex-co/bex/rulesets --input - <<'JSON'
{"name":"main history","target":"branch","enforcement":"active","bypass_actors":[],
 "conditions":{"ref_name":{"include":["refs/heads/main"],"exclude":[]}},
 "rules":[{"type":"deletion"},{"type":"non_fast_forward"}]}
JSON
```

Then update the id in this runbook. The daily `github-repo-security-drift.yml` workflow keeps an issue open while either rule is missing, and `scripts/ship-gates.sh` warns on every ship.
