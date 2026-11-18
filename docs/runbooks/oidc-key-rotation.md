# Runbook: rotate the Managed OIDC signing key

**What it is:** the key set every workspace's OIDC issuer (`https://oidc.bex.co/<tea-…>`) signs with and publishes in its JWKS. One ES256 P-256 key signs at a time, for every workspace. The keys live in the control-plane database, table `oidc_signing_keys`. Design and numbers: [ADR116 D5 and D6](../ADR116-managed-oidc.md#d6--bootstrap-and-rotation). Board: `w2/m476`.

**It is a command in the bex-api pod, not an API route.** `/api oidc-rotate-key` is an operator-run verb like `/api revoke-oauth-grant`: it never mounts on `:8090`, and the cluster access you already hold is its only credential.

## When to rotate

- **Routinely**, when you want a fresh key (for example yearly, or after a person with database access leaves). Use the normal rotation: nothing a tenant holds stops verifying.
- **At once**, when the private key may have leaked (a database dump or backup in the wrong hands). Use the emergency rotation: tokens signed with the old key stop verifying immediately, and services get working tokens again at their next refresh.

There is nothing to do when the table is empty or lost: bex-api creates a key on its own, and providers pick it up from the JWKS. Providers trust the issuer URL, never a pinned key.

## 1. Reach the cluster

```bash
bash scripts/fetch-app-kubeconfig.sh "$HOME/.kube/bex-prod.kubeconfig"   # a path outside the repo
export KUBECONFIG="$HOME/.kube/bex-prod.kubeconfig"
```

On a dev-N stack the same commands run against the local binary instead: `cd lego/backend && BEX_CP_DB_URI=… BEX_OIDC_ISSUER_URL=http://oidc.localhost:<port> go run ./cmd/api oidc-rotate-key`.

## 2. Dry run

```bash
kubectl -n bex-system exec deploy/bex-api -c api -- /api oidc-rotate-key
```

It writes nothing and prints the key set (`kid`, `state`, `activatesAt`), the key it would add (`newKey`) and when the key signing now would stop being published (`supersededRemovedAt`). `state` is one of:

| State | Meaning |
| --- | --- |
| `next` | published, signs from its `activatesAt` |
| `current` | published and signing |
| `previous` | superseded, still published so the tokens it signed keep verifying |
| `removed` | superseded 2 or more hours ago; deleted at the next reload |

## 3a. Normal rotation

```bash
kubectl -n bex-system exec deploy/bex-api -c api -- /api oidc-rotate-key --apply
```

The new key is published at once and signs **1 hour** later. The key it supersedes stays in the JWKS for **2 hours** after that, longer than any token it signed lives (1 hour), and is then deleted. Every replica reloads the set every minute; the switch happens at the same instant on all of them.

A second rotation is refused while the first one's key has not started signing (`refused: a key from an earlier rotation has not started signing yet`). Wait until its `activatesAt`.

## 3b. Emergency rotation

```bash
kubectl -n bex-system exec deploy/bex-api -c api -- /api oidc-rotate-key --emergency --apply
kubectl -n bex-system rollout restart deploy/bex-api
```

The new key signs at once and every other key is deleted, in one transaction. The restart makes every replica drop the old key now rather than at its next reload (up to a minute later). Tokens signed with the old key, including any a thief minted, stop verifying as soon as a provider re-fetches the JWKS (Render's `Cache-Control` allows 5 minutes).

## 4. Confirm

```bash
curl -s https://oidc.bex.co/tea-c0000000000000000000/.well-known/jwks.json | jq -r '.keys[].kid'
KUBECONFIG=… bash scripts/oidc-issuer-verify.sh <tea-…> <srv-…>
```

After a normal rotation the JWKS lists both kids until the old one is removed; after an emergency rotation, only the new one. `scripts/oidc-issuer-verify.sh` mints a token for a service and verifies it through the public issuer with an independent library; it must exit 0, and the `kid` it prints is the new key's once that key signs.

## Limits worth knowing

- **A replica that cannot reach the database for an hour stops signing** (it may have missed a rotation) but keeps serving the JWKS it has. Signing resumes with its next successful reload.
- **The private key is in the database and its backups.** That is the residual of ADR116 D5. Treat a lost backup as a leaked key: rotate with `--emergency`.
- **`/api oidc-test-token`** mints for the audience `bex-oidc-verify` only, which no provider accepts. It exists for the verification script, not to act as a tenant.
