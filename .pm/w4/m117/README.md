# w4 · m117 — Service ipAllowList denies allow-listed clients

**Worker:** worker4 **Goal:** a service allow-list admits listed clients and denies everyone else **Status:** todo

## Tasks (in order)

| id   | title                                                     | est | depends_on |
| ---- | --------------------------------------------------------- | --- | ---------- |
| t001 | Find and fix the service allow-list enforcement gap       | 90m | —          |
| t002 | Render parity for service allow-list semantics            | 30m | t001       |
| t003 | Simplify the code this milestone changed                  | 20m | t002       |
| t004 | Test coverage for the shipped behavior                    | 30m | t002       |
| t005 | Closeout                                                  | 20m | t004       |

## Definition of done

- A free web service with an allow-list holding the caller's exact v4 `/32` and v6 `/128` answers `200` to both `curl -4` and `curl -6` from that caller, and `403` from a non-listed source (or the closest provable deny: remove one family and show it flips to `403` while the other stays `200`).
- The enforcement path is covered by a regression that fails on today's behavior (allow-listed client refused).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` infinite loop, sweep 3, 2026-09-17 (~07:25–07:30 UTC). Workspace `bex-canary`, fixture `srv-dalpa0b00vpc73coftmg` (`qa-20260917-a735c9-web3`, deleted after the sweep). Installed `bex v0.2.1`. Sanitized repro inline in t001; no gitignored transcript carries evidence.
- **Goal linkage:** an IP allow-list that refuses its own entries makes the feature unusable (fail-closed for legitimate users). Fixing enforcement keeps bex the Render alternative users can lock down (ADR008). Touchpoints: operator ingress projection, edge source-IP strategy.
- **Expected outcome:** service allow-lists enforce per-entry as documented, with a live-verified regression.
- **Why now:** anyone enabling the feature today locks out their own clients; the longer it stands, the more "allow-list doesn't work" workarounds pile up. Filed from the CLI loop — merge with any dashboard-loop filing of the same enforcement gap instead of fixing twice.
