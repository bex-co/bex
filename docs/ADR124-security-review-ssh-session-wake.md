# ADR124: Security review — an SSH login wakes a closed agent session

- **Status**: Accepted (2026-11-23, w10/m597)
- **Trigger**: w10/m597 gives the SSH gateway its first bex-api verb that changes something. A native SSH login into an agent session whose sandbox closed asks bex-api to start it as the key's owner, and the login is held while it starts ([ADR054 D9](ADR054-open-in-zed.md#d9--an-ssh-login-wakes-a-closed-session-w10m597-2026-11-23)).
- **Lineage**: a focused pass in the gateway-verb line, after [ADR118](ADR118-security-review-scoped-api-keys.md) reviewed the key-grant verb (w9/125) and [ADR012 §9a](ADR012-auth.md#9a-workspace-enforced-two-factor-authentication-w22m402) the two-factor verb. It keeps their invariants: a gateway verb fails closed, is signed with `BEX_SANDBOX_EXEC_SECRET` and claimed once, and answers only for what the gateway itself verified.

## Scope

`lego/backend/internal/agentsession/session_wake.go` (the verb's handler and client, `InternalSessionWakePath`, the `wake-session` MAC domain); `agentsessions.Service.ResumePhase` over `Service.Resume` (`editor.go`); the resolver's wake params (`agentsessions/ssh.go` `errNoSandboxToOpen`, `sshReachable`); the gateway's held login (`sshgateway/nativessh/wake.go`, `server.go` `VerifiedPublicKeyCallback`, `serveMultiChannel`, `revalidator`, `session.go` `serveSession`); the wiring in `cmd/api/main.go` and `cmd/ssh-gateway/main.go`.

## What changed in the trust model

Before, `BEX_SANDBOX_EXEC_SECRET` let a gateway ask bex-api questions (email verification, login requirements, an API key's grant) and mint the credentials a sandbox's own traffic needs, each bound to a gateway-verified Pod. Now it also lets a gateway make bex-api start a sandbox, billed to its workspace, as the owner of any registered SSH key the gateway names by fingerprint.

Weighed against what the gateway already holds, this is small. Its role reads `ssh_keys` and `agent_sessions`, and it holds `pods/exec` in every `<ws>-sandbox` namespace (ADR054 D7), so a compromised gateway already runs code in every live sandbox. The verb adds the power to start one more sandbox for a session whose owner-key holder may start it. It runs no command and mints no credential. Binding the request to a registered key means a gateway can name only a subject that has one, and the start passes that subject's own checks.

## Verdicts

| # | Attack | Verdict | Evidence |
| --- | --- | --- | --- |
| 1 | A client that has not proved possession of a key (a `PublicKeyCallback` key query) triggers a start | Blocked | the verb is called only in `VerifiedPublicKeyCallback`, after the signature verified, the key was re-read, the owner's email was found verified, the resolver passed `can_view_sensitive` cached and fresh with the login requirements, and an API key owner's grant was found to cover Sandbox execution (`server.go`, `wakeSession`) |
| 2 | A key that may not view the session, or that names one that does not exist, starts it or learns it exists | Blocked | the resolver authorizes before reading the row; such a login is a bare rejection and calls nothing (`TestSSHWakeStartsNothingForWhatItMayNotWake`) |
| 3 | A Viewer (no `can_operate`) or a Contributor (no fresh `can_create`) starts a sandbox | Blocked | bex-api runs the owner's own `Service.Resume`, which checks both; the login keeps w10/043's banner (`TestSSHWakeVerbStartsAClosedSessionAsTheKeysOwnerPG`, `TestSSHWakeRefusalsAreToldInTheBanner`) |
| 4 | A workspace's login requirement (two-factor, Google sign-in) is skipped | Blocked twice | the gateway's resolver refuses first; bex-api's `Resume` authorizes as `{owner, ssh, the key's sign-in}`, which runs the real gate (`TestSessionWakeResumesAsTheKeysOwner`, `TestSessionWakeRefusesWhatTheOwnerMayNotStart`) |
| 5 | A key enrolled by a bex API key starts what its grant does not cover | Blocked twice | the gateway requires Sandbox execution before calling (`keyOwnerAction`); bex-api reads the grant uncached and requires Agents manage and Sandbox execution, as the key's own REST resume and shell would (`TestSessionWakeRefusesWhatTheOwnerMayNotStart`) |
| 6 | A gateway names a subject other than the key's owner, or a deleted key | Blocked | bex-api re-reads the key by fingerprint and refuses unless its owner is the subject named (`TestSessionWakeRefusesWhatTheOwnerMayNotStart`, `TestSSHWakeVerbStartsAClosedSessionAsTheKeysOwnerPG`) |
| 7 | A captured request is replayed, or presented past the clock skew | Blocked | a single-use nonce claimed in Postgres, a 30-second skew window (`TestSessionWakeIsSignedForItsOwnDomainAndRunsOnce`) |
| 8 | A signature made for another gateway verb (a bare-body question, the spawn verb) is presented to the wake | Blocked | the wake verifies under its own `wake-session` MAC domain and decodes strictly, its kind included (`TestSessionWakeIsSignedForItsOwnDomainAndRunsOnce`) |
| 9 | Repeated logins, several keys or several connections start many sandboxes, or pass the live-sandbox cap | Blocked | `resume` returns a `resuming` session unchanged and `BeginAgentSessionEditorStart` is a compare-and-set, so one start; admission runs as for a turn (`TestSSHWakeVerbStartsAClosedSessionAsTheKeysOwnerPG`: two wakes, one start; the cap refused) |
| 10 | Held logins exhaust the gateway | Bounded | a held login holds its session-limiter slot (5 per identity, 100 per process) and each held channel a channel slot, as an open shell; the pre-auth slots are released at the handshake, which the verb answers within it; the wait ends at the 16-minute start ceiling (`TestSSHWakeThatEndsWithoutASandboxSaysSoAndCloses`) |
| 11 | A member whose access or key is revoked while a login is held gets a shell once the sandbox is up | Blocked | the wait re-reads the key and re-runs the resolver every poll, and each held channel re-authorizes fully (key, email, resolver, grant) before its command runs (`TestSSHWakeEndsWhenAccessIsRevokedMeanwhile`) |
| 12 | The revalidation watchdog is switched off for a held login and stays off | Not a gap | it skips only while the wait is on, when nothing runs; once the wait ends it runs as on any connection (`TestSSHIntoAClosedSessionWakesItAndOpensTheShell` runs at a 20 ms tick) |
| 13 | A refusal message injects terminal control sequences or floods the banner | Blocked | bex-api's message is bex's own copy, told only after `Resume`'s authorization passed; the gateway prints it as one printable line of at most 300 characters (`TestWakeBannerTextIsOneBoundedPrintableLine`) |
| 14 | A rolling deploy: a new gateway asks an old bex-api, or the verb is unwired | Fails closed to the old behavior | any non-answer (404, 503, transport) gives w10/043's banner and starts nothing (`TestSSHWakeRefusalsAreToldInTheBanner`, `TestSessionWakeWithoutAnAnswerIsAnError`) |
| 15 | The audit log loses who started the sandbox | Not a gap | `Resume`'s `can_operate` check records the allowed write as the key's owner with method `ssh` and the client's address, which the gateway passes and bex-api validates as an IP (`TestSessionWakeResumesAsTheKeysOwner`) |

## Residuals

- **The creator ceiling is not applied to an SSH key enrolled by a bex API key.** Such a key authorizes as method `ssh`, not `oauth2`, so `keyCeiling` does not cap it at its creator, on the wake as on its shell since w9/125. Accepted: enrollment by API keys was refused in ADR072 F3, so only keys added before that remain, and the wake also requires the key's grant to cover Agents manage.
- **A compromised gateway can start sandboxes for any subject that owns an SSH key and may start the session.** Accepted, as weighed above: it already holds `pods/exec` into every live sandbox, and each start passes that subject's own authorization and admission and is billed and audited as theirs.
