# ADR107: Security review round 24 — the surfaces shipped since round 23

- **Status**: Accepted (2026-11-13)
- **Review**: the repository's `security-audit` skill, run 1 (`~/security-audit-skill/bex-w5/run-1`), at repository revision `5bfc00af7`. Three parallel hunts, one per plane, then validation by the reviewer. Scoped to what shipped after round 23 (ADR085, 2026-08-29); the static-site rewrite proxy had its own focused review (ADR103) and is excluded.
- **Lineage**: twenty-fourth pass in the ADR028 → … → ADR085 lineage (with the focused ADR103)
- **Work item**: `.pm/w5/m371`

## Summary

Four confirmed findings: one high and three medium. All four are fixed; one fix is partial, and its remainder is filed. No critical finding, and no finding crosses a tenant boundary by id.

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| S-1 | High | The OpenSandbox list call read only the 20 newest sandboxes (the server's default page), so every older sandbox escaped the meter (unbilled compute), the active cap, idle suspension, group teardown and by-id resolution, and suspended ones lost their snapshots to the purger | Fixed: `sandbox.Client.List` reads every page at 200, re-reads once and otherwise refuses a list whose total changed between pages, and refuses a full page with no paging data; a public-id lookup uses OpenSandbox's server-side metadata filter instead of the whole list |
| H-1 | Medium | A third-party OAuth client holding a delegated admin `bex.write` token could invite an admin, re-mint an invite or promote a member; the membership outlived the consent | Fixed: `members.Invite`, `ResendInvite` and `ChangeRole` take `AuthorizeMintClass`; the operations are classified mint |
| S-2 | Medium | A sandbox image import read a registry-declared config (and manifest) unbounded, so one request from any tenant could OOM-kill a 128 MiB bex-api replica | Fixed: the declared config size must be 1 byte to 1 MiB before it is read; source manifest reads (including a redirected one) are capped at 4 MiB |
| S-3 | Medium | In-flight imports did not count against the workspace storage cap, and imports skipped the billing gates, so a tenant could fill the shared registry | Fixed in part: a build records its declared size at creation and pending and running imports count. `store.CreateSandboxBuild` refuses a new import whose size would pass the cap, under a workspace-wide lock (concurrent imports into different groups cannot all pass) and after returning a build the group already has (a repeated import is never refused). Imports take the billing gates. A platform-wide budget sized to the registry is filed as w5/189 |

## Scope and verdicts

| Plane | Surfaces | Verdict |
| --- | --- | --- |
| Cloud agents | agent-owned delivery (ADR090), Cloud Agents MCP (ADR094), attachments (ADR095), session secrets (ADR098), skills (ADR099), automations (ADR101), child sessions (ADR102), PR feedback and `/bex` takeover, CLI agent ssh/handoff/pickup (ADR106) | No exploitable vulnerability. Every :8091 verb re-checks the live sandbox; the sandbox never holds the installation token; receive-pack is confined to the session branch; by-id verbs authorize against the session. Hardening filed as w5/186–188 |
| Sandboxes | image import and builds (w2/m358), groups (w2/m359), exec and file/run connect tokens, execution history, network and pod isolation (ADR104, ADR042) | S-1, S-2, S-3. No SSRF (`netutil.SafeDialContext`), no cross-workspace by-id access, connect tokens bound and single-use. Hardening filed as w5/189 |
| Hosting | build sources (ADR097), Postgres extensions (ADR092) and credential rotation, Key Value internal auth, webhook secret reads, Blueprint plan/approve over MCP, member mutations, account deletion (ADR086), the obs.bex.co ops gate (ADR088) | H-1. Everything else holds. Hardening filed as w5/190 |

## Decisions

- **Membership issuance is mint class.** An invite, its resend and a role change create a membership that outlives the caller's credential, the same as an API key. They now take `AuthorizeMintClass`, so only a direct Kratos session or a bex platform client (the dashboard, the CLI) passes. `ResendInvite` also gained the fresh `can_manage` re-check `Invite` already had. `scope_matrix_overrides.go` classifies the ten operations as mint, and `heuristicRequiredClass` pins them.
- **Webhook export follows the same rule.** Repointing an endpoint's URL takes `AuthorizeMintClass`, as Create does. A delegated third-party token reads the destination and signing secret redacted. A platform client and a session read them as before (w3/m282). The read side uses `core.Identity.MintClass`, the same class read from the already-authenticated identity, so a list does not pay a token introspection per row.
- **Sandbox listing fails closed.** A partial OpenSandbox list is an error, never a smaller list: the meter's terminate-missing step and the purger act on absence, so a short list is unsafe.
- **Untrusted registries are bounded before they are read.** The config read is bounded by the size the manifest declares, so the declaration is what gets checked. Layers stream and are not capped.

## Residuals

- **Accepted, unchanged:** `.pm/DO_NOT_DO.md` #PSL, #CI-RUNNERS, #RUNNER-HOSTS, #ENV-REVIEWERS, #APPROVAL-MODE.
- **Filed as hardening, not exploitable today:**
  - w5/186: a removed member's in-flight agent turns run until their turn timeout. Fixed 2026-11-13: a membership exit (remove, leave, account deletion) stops the sessions the member started in that workspace, directly or through a key it revoked, before the membership row goes (ADR024).
  - w5/187: HMAC ticket flavors that share a secret are not domain-separated. Fixed 2026-11-13: every MAC binds its flavor name; the previous release's body-only MAC is accepted until w5/191 removes it.
  - w5/188: session branches are not unique across spawn; the head SHA is self-reported; takeover protection is re-checked only once. Fixed 2026-11-13 (w5/m382): one session at work per branch (a spawned child may not take any branch a session could still push to), the completer records GitHub's head, and every takeover fast-forward re-checks protection.
  - w5/189: no platform-wide import budget; the import worker pool is shared; foreign layer URLs are kept; Zot cross-repo mounts are unchecked.
  - w5/190: Blueprint approve does not bind its plan; a Grafana session outlives ops removal; retired Postgres logins keep their sessions.
- **Needs deployment testing:** S-1 has not been observed on a live workspace with more than 20 sandboxes. Also untested live: the containerd behavior behind foreign layer URLs, and whether Grafana's break-glass login is reachable.
- **Coverage:** this is the skill's first run on the repository. A single run finds roughly half of what repeated runs find, so a second run should weight the areas this one covered least: the operator and the dashboard's server-side routes.

## Verification

Regression tests fail on the pre-fix code and pass after it:

- `members.TestMembershipIssuanceRefusesADelegatedThirdPartyToken`
- `webhooks.TestRepointAndSecretReadRequireMintCredentialClass`, plus the third-party case in `TestDestinationURLRedactedForReadOnlyOAuthAdmin`
- `sandbox.TestListReadsEveryPage`, `TestListRereadsOrRefusesAListThatMovedWhileItRead` (a create, a delete, a list that never settles) and `TestListByMetadataSendsTheServerFilter`
- `sandbox.TestConfigSizeIsCheckedBeforeTheConfigIsRead`, `TestManifestResponsesAreCapped` and `TestManifestCapMatchesManifestReadsOnly`
- `sandbox.TestImportCountsItsOwnSizeAgainstTheStorageCap`, against real Postgres

The scope-matrix guards, the real-database backend suite (`scripts/ship-gates.sh`) and `make lint-backend` are the release gates. The skill's `findings.json` validates against its schema.
