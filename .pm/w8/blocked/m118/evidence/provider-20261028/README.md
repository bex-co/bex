# m118 TLS isolation verification — 2026-10-28

The working-tree fix puts usable TLS certificate references before missing/incomplete references in both serving and redirect Ingresses. Rule order, host-to-Secret mapping and middleware names stay stable. Multi-host Kubernetes reconciliation is bounded at 30 seconds for issuance/deletion, including direct static publications and sleeping services; Secret access stays on the existing uncached client.

- [Full operator suite](operator-tests.txt): `make test`, exit 0; controller tests 155.927s with 87.6% coverage.
- [All-module lint and deadcode](lint.txt): `make lint`, exit 0; operator, backend, types and CLI each report zero issues.
- [Actual pinned provider](traefik-provider.txt): `bash scripts/verify-ingress-tls.sh`, exit 0. A disposable kind cluster installs chart 41.0.0 / Traefik v3.7.5 with the repository's base values and plugins. It replays the old reference order and observes the original certificate mismatch, then checks verified HTTPS and path/query-preserving redirects with the platform certificate pending, a custom certificate pending and one of two custom certificates pending. No TLS verification bypass is used.
- Maintained unit tests additionally cover incomplete Secrets, all-ready/all-pending ordering, an uncached tenant Secret client, issuance and deletion, a sleeping web service and a directly published static site.
- `shellcheck scripts/verify-ingress-tls.sh`, `bash -n scripts/verify-ingress-tls.sh` and `git diff --check` pass.
- Simplify's reuse, quality and efficiency reviews completed; the direct-static recheck gap and harness failure/cleanup handling were corrected.

The first harness run lost its port-forward connection and did not claim acceptance. The successful run reproduced the old failure and passed every isolation case. It deleted `bex-tls-75141`; no `bex-tls-*` cluster remains. Only the pre-existing `bex` and `bex-mgmt` clusters remain. No production fixture, DNS record, login session or kubeconfig was created this turn. The preceding Cloudflare test's exact cleanup receipts remain alongside this evidence.

[Machine-readable summary](summary.json). Production rollout is still required before [t005](../../t005.md) can close the milestone. Root `AGENTS.md` requires an explicit `$ship` before commit/push; this continuation implemented and verified the fix without committing or deploying it.
