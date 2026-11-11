# w8 · m152 — An allow-list sandbox reaches its allowed hosts by name

**Worker:** worker8 **Goal:** in a sandbox created with `--allow-egress-https <domain>`, ordinary programs (`wget`, `apk`, `curl`, `pip`, anything that resolves through libc) reach the listed domain over HTTPS, as on Render; unlisted names stay unreachable. **Status:** done (2026-11-11; all eight tasks done, Definition of done 1–6 verified on production)

## Tasks (in order)

| id   | title                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce on a cluster and choose the mechanism — **DONE**             | 40m | —          |
| t002 | Make a listed name resolve through libc in an allow-list sandbox — **DONE** | 45m | t001       |
| t003 | Blast radius: agent-session and legacy-allowlist sandboxes, glibc image — **DONE** | 30m | t002       |
| t004 | The isolation matrix probes the allow-list through the libc resolver — **DONE** | 30m | t002       |
| t005 | Render parity — **DONE**                                               | 15m | t003, t004 |
| t006 | Simplify — **DONE**                                                    | 15m | t005       |
| t007 | Test coverage — **DONE**                                               | 30m | t005       |
| t008 | Closeout (live replay on production) — **DONE**                        | 20m | t007       |

## Definition of done

1. On production, with the released `bex` CLI and a human login:

   ```text
   bex sandboxes create --allow-egress-https example.com --timeout 900 --wait -o json     # → sbx-A
   bex sandboxes exec sbx-A -- sh -c 'getent hosts example.com; wget -q -T 8 -O /dev/null https://example.com && echo LISTED-OK'
   bex sandboxes exec sbx-A -- sh -c 'wget -q -T 8 -O /dev/null https://api.github.com || echo UNLISTED-DENIED'
   bex sandboxes exec sbx-A -- sh -c 'wget -q -T 8 -O /dev/null http://example.com || echo HTTP-DENIED'
   ```

   prints an address, `LISTED-OK`, `UNLISTED-DENIED` and `HTTP-DENIED`. A `*.example.com` rule lets `www.example.com` through and not the apex.

2. `apk add --no-cache curl` works in a sandbox created with `--allow-egress-https dl-cdn.alpinelinux.org`.
3. Suspend and resume the sandbox (idle timeout, then exec): the listed name still resolves.
4. `isolation-matrix.yml` is green with a libc-resolver probe of the allow-list sandbox, and every denial it already asserts (unlisted name, exfiltration query, wrong SNI, non-HTTPS port, private target, metadata) still holds.
5. An agent session still reaches its registries and the credential gateway (no regression from a DNS change shared with agent sandboxes).
6. Fixtures stopped afterwards.

## Finding (2026-11-11 UTC, `/qa-find-bugs-cli` w8 loop sweep `2a35`)

- **Severity:** major — the feature is accepted, reported and listed as enforced, but the allowed host is unreachable by name, so `--allow-egress-https` is unusable from the base image.
- **Versions:** released `bex v0.5.0` (Render CLI v2.35.0, `24ac2d60fdb2`); repo HEAD `c4167a311`; production API revision not observable.
- **Context:** `https://api.bex.co/v1/`, workspace `bex-canary`, QA human device login, non-TTY, `-o json`/text.
- **Repro:**

  ```text
  $ bex sandboxes create --allow-egress-https example.com --timeout 900 -o json
  { "id": "sbx-dbpukcthi5vc738gj4vg", "status": "running", "plan": "starter",
    "networkPolicy": {"type":"allow-list","default":"allow-list","rules":[{"domain":"example.com","protocol":"https"}]}, … }      exit 0

  $ bex sandboxes exec sbx-dbpukcthi5vc738gj4vg -- sh -c 'wget -T 8 -O /dev/null https://example.com; getent hosts example.com; echo rc=$?; ping -c1 example.com'
  wget: bad address 'example.com'
  rc=2
  ping: bad address 'example.com'                                                    (twice, minutes apart)
  ```

- **Expected:** Render's allow-list lets a sandbox make HTTPS requests to the listed domain (`render sandboxes create --allow-egress-https api.github.com`, pinned help; ADR104 § network policy). **Actual:** every libc lookup of the listed name fails; nothing is reachable by name.
- **What does work in the same sandbox** (so the data path and the policy are right, only name resolution fails):

  ```text
  nslookup -type=a example.com          → 104.20.23.154, 172.66.147.243          (busybox nslookup asks the absolute name)
  getent hosts example.com.             → resolves                               (trailing dot = absolute name)
  wget -O /dev/null https://example.com./   → 577 bytes saved, exit 0
  nslookup example.com.tea-…-sandbox.svc.cluster.local.   → ** server can't find …: REFUSED
  nslookup example.com.svc.cluster.local.                  → REFUSED
  nslookup api.github.com.                                 → REFUSED            (unlisted, correct)
  ```

  `/etc/resolv.conf` in the sandbox: `search <ws>-sandbox.svc.cluster.local svc.cluster.local cluster.local`, `nameserver 10.96.0.10`, `options ndots:5`.

- **Controls:** an `allow-all` sandbox created in the same minute resolves and fetches `https://example.com`, `https://api.github.com`, `http://example.com`; a `deny-all` sandbox reaches nothing and resolves nothing.
- **Attribution:** server / infrastructure (the per-sandbox Cilium policy plus the pod's resolver configuration). Not the CLI: the request body is upstream's and the server echoes it.
- **Root cause (traced, cluster side not inspected):** `lego/backend/internal/sessionegress/sandbox.go:239` renders the policy as `dnsRule(domainSelectors(domains))` + `fqdnRule(…)`, and `policy.go:520-531` `dnsRule` admits DNS queries only for the exact listed names. With `ndots:5`, libc first asks `example.com.<ws>-sandbox.svc.cluster.local`, then the other two search suffixes. Cilium's DNS proxy answers each with `REFUSED` (observed above). The base image is `docker.io/library/alpine:3` (`lego/backend/cmd/api/config.go:557`), and musl treats a non-NXDOMAIN failure of a search candidate as a failed lookup instead of moving on to the absolute name. The admission policy pins the same shape: `deploy/gitops/base/bex-api-namespace-admission.yaml:309-314` requires every DNS name to equal a `toFQDNs` name.
- **Why the m283 live drill passed:** `isolation-matrix` (w32/m283 t004) proves "that name over HTTPS with SNI allowed", but its probe does not resolve through the libc search path.
- **Fix options (t001 chooses, by test on a cluster):**
  - **A — resolver config:** give sandbox pods `dnsConfig.options: [{name: ndots, value: "1"}]` in `deploy/opensandbox/batchsandbox-template.yaml`, so a dotted name is asked as an absolute name first. No policy or admission change. Must confirm OpenSandbox carries `dnsConfig` from the template onto the pod, and that single-label and cluster names the platform relies on (`bex-ssh-gateway.bex-system.svc.cluster.local`, already fully qualified) still resolve.
  - **B — deterministic search expansions:** for each listed name also admit the three search-suffixed names in the DNS rule only (never in `toFQDNs`), so kube-dns answers NXDOMAIN and libc moves on. ADR042's port table already uses this shape for the opensandbox-server pod ("its deterministic resolver search expansions"). Needs the admission CEL at `bex-api-namespace-admission.yaml:309-314` and `scripts/gitops-validate.sh` to accept exactly those names; `Rules()` (`sandbox.go:164-167`) reads rules back from `toFQDNs`, so reads are unaffected.
  - **C — Cilium answers NXDOMAIN instead of REFUSED** for a denied query (`tofqdns-dns-reject-response-code: nameError`). Cluster-wide; where Cilium's config lives was not found in this repo.
- **Blast radius:** every sandbox DNS policy uses the same exact-name rule: Render allow-list sandboxes (`sandbox.go`), agent-session setup/agent phases (`policy.go:442`), and `sandbox-egress-legacy-allowlist` (`deploy/gitops/base/tenant-node-egress.yaml:240-254`). Agent sessions run a different image; whether its resolver (glibc) survives `REFUSED` was not tested (t003).
- **Unverified:** the Cilium DNS proxy as the source of `REFUSED` (inferred from the policy and the answers, not from Hubble); behavior with a glibc image; wildcard rules (`*.example.com`) were not exercised; `apk` was not run (it fails at the same lookup).
- **Dedupe:** no match in `.pm/` (open or done) for `bad address`, `ndots`, `musl`, `search domain` near sandboxes; w32/m283 (done 2026-11-08) shipped the feature; w8/244 (plan sizes) and w32/008 (help links) are different findings.
- **Cleanup:** `sbx-dbpukcthi5vc738gj4vg` and the two control sandboxes were stopped (`terminated`) in the same sweep.

## Mechanism chosen (t001, 2026-11-11 UTC): A — `ndots:1` in the BatchSandbox template

Reproduced on production with the released `bex v0.5.0` and the QA human login, workspace `bex-canary`, in two sandboxes created in the same minute: `sbx-dbpvrnqk10fs73dvo62g` (allow-list `example.com`, `*.iana.org`) and `sbx-dbpvrp2k10fs73dvo63g` (allow-all). Both were terminated afterwards and the workspace's sandbox list is empty.

- **Where `REFUSED` comes from (observed).** The same query to the same resolver (`10.96.0.10`) gets two answers that differ only by the sandbox's policy:

  ```text
  allow-list   nslookup -type=a example.com.svc.cluster.local.   → REFUSED     (also example.com.cluster.local.)
  allow-all    nslookup -type=a example.com.svc.cluster.local.   → NXDOMAIN    (also example.com.cluster.local.)
  ```

  CoreDNS answers `NXDOMAIN`; the refusal is added by the allow-list sandbox's DNS rule. This host has no cluster credentials, so no Hubble flow was read; the isolation matrix already records Hubble evidence for a denied DNS query.

- **A works on the real DNS path.** In the allow-list sandbox, as root, `/etc/resolv.conf` was rewritten from `options ndots:5` to `options ndots:1` (what the kubelet writes for the `dnsConfig` option). Before: `getent hosts example.com` rc 2, `wget: bad address 'example.com'`. After:

  ```text
  getent hosts example.com      → 2606:4700:10::6814:179a     wget https://example.com      → LISTED-OK
  getent hosts www.iana.org     → resolves (CNAME followed)   wget https://www.iana.org/    → WILDCARD-OK
  getent hosts iana.org         → rc 2                        wget https://iana.org/        → bad address   (apex of *.iana.org)
  getent hosts api.github.com   → rc 2                        wget https://api.github.com   → bad address   (unlisted)
  wget http://example.com       → download timed out                                                          (plain HTTP)
  getent hosts kubernetes       → rc 2                                                                        (single label still uses the search list, refused)
  ```

- **`dnsConfig` reaches the Pod (read in code, to be confirmed by the live replay).** `opensandbox-server` 0.2.2 deep-merges the template under the runtime manifest (`services/k8s/template_manager.py` `_deep_merge`); the runtime Pod spec sets only `automountServiceAccountToken`, `initContainers`, `containers`, `volumes`, `runtimeClassName`, `imagePullSecrets` and a platform `nodeSelector` (`batchsandbox_provider.py:208-243`), so a template `dnsConfig` survives, like `tolerations` does today. The BatchSandbox CRD keeps unknown fields under `spec.template`, and `bex-sandbox-pods` admission does not look at DNS settings.
- **Libc matrix (local, a resolver that answers `REFUSED` for every unlisted name; search list as in a sandbox):**

  | Image (libc)                             | `ndots:5`                                    | `ndots:1`                     |
  | ---------------------------------------- | -------------------------------------------- | ----------------------------- |
  | `alpine:3` (musl), the base image        | every listed name fails                      | listed resolve, unlisted fail |
  | `node:22-bookworm-slim` (glibc), agent base | listed and the gateway name resolve, unlisted fail | the same                |

- **Rejected.** B (search-suffixed names in the DNS rule): three times the names, a looser `bex-api-sandbox-egress` grammar and a policy tied to the Pod's search list, for the same result. C (Cilium answers `NXDOMAIN`): a cluster-wide agent option set only by `app-cluster.yml`'s Helm install; it would change every FQDN policy's answer and needs a Cilium rollout on production. Neither is filed as a follow-up: A covers every resolver tested.
- **Rollout.** Argo CD's `opensandbox-server` Application tracks `main` at `deploy/opensandbox`, so the template reaches production on push, not at the next image deploy. Sandboxes created before it keep `ndots:5` until they are recreated.
- **Agent-template probe did not run.** A `template: agent` sandbox with an allow-list (`sbx-dbpvt12p92ls73cjis00`, then `sbx-dbpvtiip92ls73cjisag`) went `errored` within seconds without a session to drive it, so no exec was possible; both were terminated. The agent row above therefore comes from the agent image's base, not from a production agent Pod.

## Closeout (t008, 2026-11-11 UTC): Definition of done on production

Shipped `348bd77a5` at 05:46 UTC. Argo CD's `opensandbox-server` Application tracks `main`, and the lifecycle server restarted with the new template at about 05:53 UTC; a sandbox created at 05:52:45 still had `ndots:5`, one created at 06:02:01 had `options ndots:1`. Released `bex v0.5.0`, QA human device login, workspace `bex-canary`.

1. **Passes.** `sbx-dbq09mak10fs73dvo9e0` (`--allow-egress-https example.com --allow-egress-https '*.iana.org'`):

   ```text
   getent hosts example.com                     → 2606:4700:10::ac42:93f3  example.com
   wget -q -T 8 https://example.com             → LISTED-OK
   wget -q -T 8 https://api.github.com          → wget: bad address 'api.github.com'   UNLISTED-DENIED
   wget -q -T 8 http://example.com              → wget: download timed out             HTTP-DENIED
   wget -q -T 8 https://www.iana.org/           → WILDCARD-SUBDOMAIN-OK
   wget -q -T 8 https://iana.org/               → wget: bad address 'iana.org'          WILDCARD-APEX-DENIED
   wget -q -T 8 https://1.1.1.1/                → wget: download timed out             (a literal IP stays denied)
   ```

   `bex sandboxes list` reads the rules back as sent: `*.iana.org` and `example.com`, `https`.

2. **Passes.** `sbx-dbq0a6ak10fs73dvoa5g` (`--allow-egress-https dl-cdn.alpinelinux.org`): `apk add --no-cache curl` → `OK: 13.1 MiB in 25 packages`, rc 0; the installed `curl https://example.com` → `Could not resolve host: example.com (DNS server refused query)`.
3. **Passes.** Sandbox A (`--idle-timeout 300`) read `suspended` at 06:08:46; an exec resumed it: `options ndots:1`, `getent hosts example.com` resolves, `LISTED-OK-AFTER-RESUME`, `UNLISTED-DENIED-AFTER-RESUME`.
4. **Passes.** `isolation-matrix.yml` run `42971611034` on `f72f8b4b7` is green: `PASS: approved FQDN through the libc resolver allowed`, `PASS: approved FQDN fetched by name over HTTPS allowed`, and every denial it already asserted (unapproved name, DNS-exfiltration query, wrong SNI, non-HTTPS port, private target, metadata). The first run after the fix, `42969818556`, had the same passes and then failed on an unrelated check that w22/m311 had added the day before and that had never run in CI: it listed the default sandbox group with a developer's token and required another member's sandbox in the answer. `f72f8b4b7` (`fix(ci)`) reads that list as the workspace admin; the run opened issue #137 and the green run closed it.
5. **Passes, from a production session I did not start.** `bex-canary` has no model key, so I could not run a session there. Agent session `ags-dbq08e2p92ls73cjitp0` in workspace `bex` was started by an automation at 05:59:20 UTC, after the rollout. Its transcript (first 200 parts, read with the QA user's membership) shows the repository cloned through the gateway, model turns through the gateway's model proxy (model cost recorded), and 50 `go: downloading …` lines from the Go module registry; it contains no name-resolution error of any kind. The session ended `failed` with `ACP turn exceeded 1800000ms`, the 30-minute turn limit of a long routine, not a network fault. The matrix's own agent-phase probes (`setup-phase package registry`, `agent-phase package registry`, `agent-phase GitHub baseline`) pass on Pods from the new template.
6. **Done.** Every sandbox this milestone created is terminated and `bex sandboxes list` in `bex-canary` is empty: `sbx-dbpvrnqk10fs73dvo62g`, `sbx-dbpvrp2k10fs73dvo63g`, `sbx-dbpvt12p92ls73cjis00`, `sbx-dbpvtiip92ls73cjisag`, `sbx-dbq02map92ls73cjit40`, `sbx-dbq03b2k10fs73dvo7l0`, `sbx-dbq041qp92ls73cjit8g`, `sbx-dbq04pip92ls73cjitb0`, `sbx-dbq05bik10fs73dvo7s0`, `sbx-dbq09mak10fs73dvo9e0`, `sbx-dbq0a6ak10fs73dvoa5g`.

**Also confirmed:** two sandboxes created before the rollout kept `ndots:5` and still failed the lookup after a suspend and resume, as ADR042 says: a BatchSandbox keeps the template it was created from.

**Seen on the way, filed by a teammate as w8/248:** while the lifecycle server restarted for this rollout (about 05:53 UTC), `bex sandboxes stop` and `GET /v1/sandboxes` answered `500 internal error`.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` w8 loop, sweep `2a35`, 2026-11-11 UTC (finding above).
- **Goal linkage:** Render parity for sandboxes (ADR104 § network policy, ADR042 § Sandbox network policies): a restricted sandbox that can still reach its package registry or API is the reason the allow-list exists.
- **Expected outcome:** `--allow-egress-https` works for real programs; the isolation matrix would catch a regression.
- **Why now:** the allow-list shipped on 2026-11-08 and `bex v0.5.0` exposes the flag to every customer; today it silently produces a sandbox with no usable network.
- **Render parity included:** the fix changes observable sandbox network behavior behind REST/GraphQL/MCP `networkPolicy`.
