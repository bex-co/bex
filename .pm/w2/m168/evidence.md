# w2/m168 evidence

## Pre-rollout checks (2026-10-06, read-only against `hetzner-prod`)

- **Card gate.** `bex-stripe` Secret `BEX_REQUIRE_PAYMENT_METHOD=all`. The 045 note claimed the gate was empty; that was a misread, because `kubectl get deploy` can't show a `secretKeyRef` value.
- **Consent gate wiring.** The dashboard Deployment reads `BEX_OPS_ROLE_TOKEN` from Secret `bex-ops`, which exists and has that key. bex-api already serves `/internal/ops-role` with the same bearer, so `/internal/identity-claims` mounts on deploy.
- **SSH gateway wiring.** `bex-ssh-gateway` has `BEX_SANDBOX_EXEC_SECRET`, and its other :8091 URLs point at `bex-api.bex-system.svc`. The default `BEX_IDENTITY_VERIFICATION_API_URL` matches.
- **SSH key owners.** There are 2 keys:
  - one with a verified human owner;
  - one owned by `bex-bootstrap` (`m39-exact`, a w2/m39 acceptance key). Kratos admin answers 404 for that subject, which classifies it as a machine and exempts it.
- **Kratos admin 404 shape.** Probed for both a non-UUID subject (`bex-bootstrap`) and an unknown UUID; both return 404, confirming the human/machine split.
- **Unverified members of first-party or active workspaces.**
  - Most are `example.com` QA fixtures.
  - Four real addresses (redacted `on***@gmail.com`, `rg***@gmail.com`, `jj***@gmail.com`, `mf***@bex.co`) are unverified.
  - The login backstop (`require_verified_address`) already refuses them any _new_ session. The gate newly affects only their still-live sessions and CLI refresh tokens.
  - Per the user decision (no grandfathering), they recover by verifying through the resend flow. Their apps keep running, because the gate is API-only.
- **Rollout ordering risk.** If the dashboard rolls out before bex-api has the identity-claims verb, consent fails closed: OAuth logins are refused for minutes until bex-api is up.

## Local acceptance

- The local CAPD app cluster had disappeared (only `bex-mgmt-control-plane` was running), so every dev-N stack was down.
- `scripts/mock-cluster.sh` reprovisioned the app cluster nodes. It then hung on `helm upgrade --install kubelet-csr-approver oci://ghcr.io/...`: helm invokes `docker-credential-osxkeychain get`, which waits on a locked macOS Keychain. An empty `DOCKER_CONFIG`/`HELM_REGISTRY_CONFIG` did not bypass it.
- Three runs were tried and stopped. The dev-2 end-to-end journey (t009) is gated on that Keychain prompt.
