# w4 · m117 — Service ipAllowList denies allow-listed clients

**Worker:** worker4 **Goal:** a service allow-list admits listed clients and denies everyone else **Status:** done 2026-09-21 — closed as a duplicate of `w1/m150`, which owns the fix; this milestone's unique evidence is merged into `.pm/w1/blocked/m150/README.md`

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- | --- |
| t001 | Find and fix the service allow-list enforcement gap | 90m | — | — **DONE (merged into w1/m150)** |
| t002 | Render parity for service allow-list semantics | 30m | t001 | — **DONE (merged into w1/m150)** |
| t003 | Simplify the code this milestone changed | 20m | t002 | — **DONE (merged into w1/m150)** |
| t004 | Test coverage for the shipped behavior | 30m | t002 | — **DONE (merged into w1/m150)** |
| t005 | Closeout | 20m | t004 | — **DONE (merged into w1/m150)** |

## Definition of done

- A free web service with an allow-list holding the caller's exact v4 `/32` and v6 `/128` answers `200` to both `curl -4` and `curl -6` from that caller, and `403` from a non-listed source (or the closest provable deny: remove one family and show it flips to `403` while the other stays `200`).
- The enforcement path is covered by a regression that fails on today's behavior (allow-listed client refused).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` infinite loop, sweep 3, 2026-09-17 (~07:25–07:30 UTC). Workspace `bex-canary`, fixture `srv-dalpa0b00vpc73coftmg` (`qa-20260917-a735c9-web3`, deleted after the sweep). Installed `bex v0.2.1`. Sanitized repro inline in t001; no gitignored transcript carries evidence.
- **Goal linkage:** an IP allow-list that refuses its own entries makes the feature unusable (fail-closed for legitimate users). Fixing enforcement keeps bex the Render alternative users can lock down (ADR008). Touchpoints: operator ingress projection, edge source-IP strategy.
- **Expected outcome:** service allow-lists enforce per-entry as documented, with a live-verified regression.
- **Why now:** anyone enabling the feature today locks out their own clients; the longer it stands, the more "allow-list doesn't work" workarounds pile up. Filed from the CLI loop — merge with any dashboard-loop filing of the same enforcement gap instead of fixing twice.

## Outcome (2026-09-21) — duplicate, merged into `w1/m150`

This milestone is closed without a code change of its own, because the fix already has an owner and closing it any other way would mean fixing the same bug twice. Its own Source section anticipated exactly this: _"Filed from the CLI loop — merge with any dashboard-loop filing of the same enforcement gap instead of fixing twice."_ That filing exists: **`w1/m150`**, from the `/qa-find-bugs` dashboard loop, 2026-09-14 pass 11.

**Same bug, already diagnosed to object level.** t001 asked for the root cause "with object-level evidence (no 'likely depth' hedging)". `w1/m150` has it, and it is t001's own prime suspect, confirmed:

- `cidrMiddlewareSpec` (`lego/operator/internal/controller/database_controller.go:563-569`) emits Traefik `ipAllowList.sourceRange` with **no `ipStrategy`**, so the match runs against the TCP peer.
- The TCP peer is the Hetzner load balancer: `infra/terraform/main.tf:190,208,226` keep `proxyprotocol = false` on the `http`/`https` listeners, and all 50 sampled request-log records read `ClientHost: 10.10.0.7`.
- Therefore every listed client CIDR is wrong by construction — and `10.10.0.7/32` would admit the entire internet. The failure is symmetric across IPv4 and IPv6, which is precisely what this milestone's sweep-3 repro observed.

**Part of the fix has already shipped**, and the rest is not ours to apply. `w1/m150` t001 landed in `acf1731c9` (Traefik's `web`/`websecure` entrypoints trust PROXY protocol from `10.10.0.7/32` only, verified on v3.7.5 that a headerless connection is still served, so the load balancer's health checks keep passing) and t003 (the blast-radius pass over every consumer of the client address) is done. What remains is t002 — flipping the Hetzner listeners — which is parked because **committing to `infra/` is `terraform apply -auto-approve` on push**, and flipping the listeners before Traefik expects PROXY headers is a full HTTP(S) outage. That is a user decision and a user-held credential, which is why `w1/m150` sits in `.pm/w1/blocked/m150/`.

**Nothing was lost.** This milestone's evidence that `w1/m150` did not have — the CLI write path proven innocent, the bare 9-byte `Forbidden` identifying Traefik rather than the app as the denier, and above all the **IPv6 `/128` half, which rules v6 handling out as a separate cause** (pass 11 was IPv4 only) — is appended to `.pm/w1/blocked/m150/README.md` as an "Independent corroboration" section, along with the fixture id and the `--clear-ip-allow-list` scope note for its live re-verify. Its t006 regression should assert both address families; the PROXY-protocol fix covers both at once.

The w4 tasks t002–t005 (parity, simplify, tests, closeout) map one-to-one onto `w1/m150` t004–t007 and are covered there.
