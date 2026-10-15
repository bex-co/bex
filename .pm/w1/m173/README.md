# w1 · m173 — Free web services go to sleep again: the production canary was held awake by a local cluster's ACME self-check

**Worker:** worker1 **Goal:** the production tenant-view canary sleeps after its idle window again, nothing local can generate certificate-validation traffic against production hosts, and the canary's failure text points at this class of cause. **Status:** in progress (t001–t004, t006, t007 done; t005 production verification pending)

## Tasks (in order)

| id   | title                                                                                                    | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Diagnose why the canary never slept (2026-10-07 → 10-15) — **DONE**                                       | 45m | —          |
| t002 | Local clusters sign their own certificates: no ACME, so no self-check traffic against production — **DONE** | 30m | t001       |
| t003 | Guard: `gitops-validate.sh` fails if the local render carries an ACME ClusterIssuer — **DONE**             | 20m | t002       |
| t004 | Canary stage 4 names the never-slept cause; FUTURE-MAYBE w3/040 records the finding — **DONE**             | 20m | t003       |
| t005 | Production: the canary sleeps between requests; three green canary runs; issue #87 closed                 | 30m | t004       |
| t006 | Simplify — **DONE**                                                                                                 | 15m | t005       |
| t007 | Test coverage — **DONE**                                                                                            | 30m | t005       |
| t008 | Closeout                                                                                                 | 10m | t007       |

## Definition of done

- **No local ACME.** The local overlay's ClusterIssuers render self-signed with no ACME solver (`scripts/gitops-validate.sh` asserts it, and fails with the patch removed); prod's issuers are untouched.
- **No traffic from the Mac.** While the local cluster runs, production request logs for `hello-go.onbex.co` show no `/.well-known/acme-challenge/` requests, and the canary records `service_hibernated` within ~16 minutes of its last real request.
- **The alarm works.** Three consecutive `ssh-edge-liveness.yml` runs pass `tenant-view-liveness` with a measurable stage-1 wake and a fresh `service_woken`; GitHub issue #87 is closed with the run ids.
- **Impact recorded.** Only the canary was affected: the two other free web services in `bex-canary` slept normally all week (evidence below).

## Evidence (t001, 2026-10-15)

- **The canary never slept.** Its events (`GET /v1/services/srv-daif6dsmg29s73d1umvg/events`) jump from `service_woken` 2026-10-07T06:00:07Z straight to `service_hibernated` 2026-10-15T03:00:42Z; before and after, it slept ~15 minutes after each wake. The two other free web services in the same workspace (`srv-db09vbgehcmc739j12i0`, `srv-db09u90ehcmc739j12cg`) slept and woke normally throughout, so this was not an operator regression (an operator test of the failed-build shape also passed).
- **What kept it awake.** Its request logs show `162.224.81.143` (an AT&T residential line, `sbcglobal.net`, the Mac that runs the local kind/CAPD cluster) requesting `hello-go.onbex.co/.well-known/acme-challenge/ehTFV1KUF-0XGTgseqCHNtRPVpsY7popUYNyDHW_QoI` every ~10s from 2026-10-07T05:59:49Z to 2026-10-15T02:44:43Z. The first request reached the activator (503) and woke the service; hello-go answers every path 200, so each check failed and repeated. The service slept 16 minutes after the last one.
- **Where it came from.** The local kind cluster `bex` held Challenge `tea-daul85i9086o6jitc8qg-hello-go-tls-1-2369103061-382143810` (created 2026-10-01T01:15:21Z, token `ehTFV1KUF-…`, "Waiting for HTTP-01 challenge propagation: did not get expected response") for a local App named `hello-go`: the local operator runs `lego/operator/config/default`, whose `BEX_BASE_DOMAIN=onbex.co` and `BEX_CLUSTER_ISSUER=letsencrypt-prod` gave it the production hostname and a real Let's Encrypt issuer. The self-check only runs while the Mac's cluster is up; it resumed at 09:58:12Z today when the cluster was restarted for this milestone, and stopped at 10:01:14Z when the local Certificate was deleted.
- **Same leak, other hosts.** The same local cluster held 13 more pending challenges for production names (`api.bex.co`, `ssh.bex.co`, `dashboard.bex.co`, and `auth.bex.co` + `oauth.bex.co` in each `dev-N-auth`), each polling production since 2026-09-30/10-02 whenever the cluster ran.
- **Local cleanup done (2026-10-15 10:01Z):** removed `cert-manager.io/cluster-issuer` from the local `tea-daul85i9086o6jitc8qg-hello-go` Ingress and deleted its Certificate (Order, Challenge and solver Ingress went with it). The other challenges stop once the local Argo applies t002's issuer change.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-10-15, item 1 (approved by the user the same day); the CI/incident sweep found the red canary. t001 disproved the filed premise (an operator auto-hibernate regression) and found the cause above; the milestone was rescoped on 2026-10-15 accordingly.
- **Goal linkage:** V0 roadmap 1 and 5 (free-tier sleep economics) and 2 (basic obs: the canary must alarm on real faults).
- **Expected outcome:** the canary sleeps and alarms correctly; no developer machine can keep a production service awake or poll production hosts through cert-manager.
- **Why now:** the leak recurs every time the local cluster runs (it did again today), and a canary red for the wrong reason hides real failures.
- **Render parity omitted:** no REST/GraphQL/MCP/UI change; this is local GitOps configuration and a probe script.
- **Related:** `.pm/FUTURE-MAYBE.md` w3/040 (unexplained canary wakes) gains this as a confirmed traffic source. `w4/blocked/m164` and `w1/blocked/124` are unrelated to the cause.
