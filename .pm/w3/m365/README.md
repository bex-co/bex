# w3 · m365 — Leaked credentials are refused at push time on the public repository, and the setting cannot silently turn off

**Worker:** worker3 **Goal:** a push to `bex-co/bex` that carries a credential in a format GitHub recognizes is refused before it lands. `main` cannot be deleted or force-pushed. A daily check opens an issue when any of these settings regresses. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Enable repository secret scanning, push protection, non-provider patterns and validity checks; record the before and after state | 15m | — |
| [t002](t002.md) | Disposition every alert the first scan raises: rotate live credentials, close the rest with a reason, never print a secret value | 60m | t001 |
| [t003](t003.md) | A `main` ruleset that blocks deletion and non-fast-forward pushes, with no PR requirement and no bypass, plus the sanctioned-rewrite runbook | 30m | t001 |
| [t004](t004.md) | A drift check for the security settings and the ruleset, with tests and a daily workflow that opens an issue | 60m | t003 |
| [t005](t005.md) | Ship skill, AGENTS.md and ADR019: a push refused by push protection is fixed by removing and rotating the secret, never by a bypass | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

- `gh api repos/bex-co/bex -q .security_and_analysis` shows `secret_scanning`, `secret_scanning_push_protection`, `secret_scanning_non_provider_patterns` and `secret_scanning_validity_checks` all `enabled`.
- A test push of a branch carrying a recognized dummy token pattern (a provider's documented sample format, never a real credential) is refused by push protection, and the branch never reaches the remote. Record the refusal message, not the token.
- `gh api repos/bex-co/bex/rules/branches/main -q '.[].type'` lists `deletion` and `non_fast_forward`. An ordinary fast-forward push to `main`, including the image-pin bot's `chore(deploy): pin platform images` commit, still lands.
- `gh api 'repos/bex-co/bex/secret-scanning/alerts?state=open'` returns no alerts. Every closed alert carries a resolution and a comment. Each one that was a live credential has a rotation record (date, owner, where the new value lives) with no secret value in it.
- The drift script exits 1 on fixtures where any one setting is disabled or the ruleset is missing or weakened, and exits 0 against the live repository. Its daily workflow runs green on `main`.
- The ship skill, AGENTS.md and ADR019 §Decision 5 say what to do when push protection refuses a push.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-11-12, item 1, filed by `/pmx them all to w3`. Re-checked 2026-11-12 against `origin/main` `24bea6c5c`.
- **Evidence:**
  - `gh api repos/bex-co/bex -q '.visibility, .security_and_analysis'` → `public`, with `secret_scanning`, `secret_scanning_push_protection`, `secret_scanning_non_provider_patterns`, `secret_scanning_validity_checks` and `dependabot_security_updates` all `disabled`.
  - `gh api repos/bex-co/bex/branches/main -q .protected` → `false`. The only ruleset is `bex-cli release tags` (target `tag`).
  - The only secret scan today is the gitleaks job in `.github/workflows/deploy.yml` (w7/002), which runs after the push has landed. By the time it fails, the credential is in public history.
  - About 900 commits a week land on `main`, mostly from agents whose sessions hold `.env`, kubeconfigs and Stripe and Hetzner tokens.
  - The ship skill already forbids force-pushing `main` (`.claude/skills/ship/SKILL.md:120,134`). `main` was rewritten deliberately twice on 2026-11-04 (AGENTS.md, #NO-ARTIFACTS-IN-GIT), which is why t003 records a sanctioned-rewrite runbook instead of a permanent ban.
- **Goal linkage:** ADR008 roadmap item 7 (security review), ADR019 infra-credential custody, and enforcement of the AGENTS.md rule "Never commit/print `.env` or `*.kubeconfig`".
- **Expected outcome:** a credential in a recognized format is refused at push time instead of being found after it is public. A branch-protection or scanning regression shows up as an issue within a day.
- **Why now:** a leak into a public repository cannot be undone. Agent push volume keeps rising, and these features are free for public repositories. GitHub's user-level push protection covers only accounts that kept its default. The repository setting covers every pusher, bots included.
- **Decision state:** none open. The settings are repository configuration. Filing this milestone through `/pmx` is the user's approval to schedule the worker to change them. Rotations that need a provider console or a person are parked in `blocked/` with exact steps (t002).
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard surface changes. This is repository security configuration and tooling.
