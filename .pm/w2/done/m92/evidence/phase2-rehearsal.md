# w2/m92 · Phase 2 rehearsal (scratch App)

**Date:** 2026-09-08 · **Cluster:** `hetzner-prod` · **Scope:** disposable first-party App only (not a tenant worklist App)

## Scratch App

| Field | Value |
| --- | --- |
| Namespace / name | `tea-d98210cbbpdc73dcrkvg` / `tea-d98210cbbpdc73dcrkvg-m92-scratch` |
| Workspace label | `tea-d98210cbbpdc73dcrkvg` |
| Spec | image-backed `traefik/whoami:v1.10.2`, `expose: false`, 1 replica |
| STOP | Does not apply (runbook: rehearse on scratch, not a tenant) |

## Procedure

1. Created labeled scratch App; waited until `phase=Running`.
2. Port-forwarded in-cluster Zot (`svc/zot` → `127.0.0.1:15000`); authenticated as `bex-builder` (password from `bex-build/bex-registry-push`, not recorded).
3. Planted legacy-shaped tag: `skopeo copy … docker://127.0.0.1:15000/<APP>:gen-1` (`--override-os linux --override-arch amd64`).
4. Pre-migrate HTTP via `svc/<APP>` port-forward: whoami response OK.
5. Ran scoped `registry-migrate --apply` (single `--app`, not `--all`):
   - plan: `tag gen-1 copy` + `tombstone: yes (no blob delete)`
   - digest-preserving copy to `W/A:gen-1`
   - stamped `app.bex.co/identity-tombstone=true`
6. Verified:
   - `DIGEST_OK` — src legacy = dest scoped = `sha256:7000846753fcc36eb2a1a3a21fe897da09c71e1d3b5381f912ea6e6e1c8871b6`
   - legacy tags include `gen-1` and `bex-tombstone`
   - `LEGACY_BLOB_REMAINS=yes` (no blob delete)
7. Post-migrate HTTP OK; `rollout restart` + status OK; App still `Running`; post-redeploy HTTP OK.
8. Deleted scratch App (`kubectl delete app …`).

## Result

| Criterion | Evidence |
| --- | --- |
| Scoped `--apply` copy + digest-verify + tombstone | `DIGEST_OK`, `tombstone=true`, `bex-tombstone` tag |
| Zero blob deletes | `LEGACY_BLOB_REMAINS=yes` |
| Dual-read / serve pre- and post-redeploy | whoami HTTP before migrate, after migrate, after restart |
| No tenant worklist App touched | only `…-m92-scratch` |

## Notes for Phase 2 (tenant)

- Workstation now has `skopeo` (Homebrew). Use Zot port-forward + builder secret the same way.
- Operator `BEX_REGISTRY_DUAL_READ` is still **unset** on prod — enable for the supervised tenant migration window before Phase 2 `--apply` on the t001 worklist (see Phase 1 inventory).
