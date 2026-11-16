# Runbook: cut off a stolen OAuth token (one human's grant)

**What it is:** an operator command that revokes one person's OAuth grant to one client, or to every client, at Hydra **and** advances the shared `oauth_revocations` marker, so every bex-api replica refuses the token at its **next request** instead of when its 5 s introspection cache entry expires. Design and measurements: [ADR012 §8d](../ADR012-auth.md#8d-revocation-latency-w21085). Board: `w21/090`.

**It is a command in the bex-api pod, not an API route.** `/api revoke-oauth-grant` is an operator-run verb like `/api migrate-container-policy` and `/api platform-rebuild`: it carries no caller identity, so it never mounts on :8090, and the cluster access you already hold is its only credential. Hydra's admin API, the control-plane database and the ops workspace id are in the pod's environment; nothing is copied to a laptop. It was chosen over a script (a laptop would need Hydra's admin API and a database password, two new secrets) and over an admin route behind the ops-workspace gate (the operator would need a bearer token of an ops member, a third credential, and a route on the public listener whose only protection would be that bearer).

## When to use it

A person's device, token or session was lost, stolen or leaked, and waiting out the cache window is not acceptable: a stolen CLI login (the Render CLI client's tokens last seven days), a leaked mobile token, a compromised MCP agent grant. The revocation reaches **human** grants (a subject that is a Kratos identity). It does not reach:

- **An API key** (`client_credentials`; the token's subject is its own client id). The command refuses a subject equal to the client. Revoke the key in Settings, API Keys, or `DELETE /v1/api-keys/{id}`: that path already writes the marker and drops the key's workspace binding, with no window.
- **A Kratos browser session** (the dashboard cookie). It is never cached: revoke it in Kratos (Settings, active sessions, or `DELETE /admin/identities/{id}/sessions`) and the next request is refused.
- **A request already running.** It finishes; a presigned object URL stays valid until its own expiry.

## 1. Reach the cluster

```bash
HCLOUD_TOKEN="$(grep '^HCLOUD_TOKEN=' .env | cut -d= -f2-)" \
  bash scripts/fetch-app-kubeconfig.sh "$HOME/.kube/bex-prod.kubeconfig"   # a path outside the repo
export KUBECONFIG="$HOME/.kube/bex-prod.kubeconfig"
```

Never print the token or the kubeconfig. (`scripts/fetch-app-kubeconfig.sh` reads the SSH key `~/.ssh/bex`; see [operator-kubeconfig-isolation.md](operator-kubeconfig-isolation.md) for the narrower operator kubeconfig.)

## 2. Find the subject

The subject is the person's Kratos identity id. From an email:

```bash
kubectl -n auth port-forward service/kratos-admin 14434:80 &
curl -s "http://127.0.0.1:14434/admin/identities?credentials_identifier=person@example.com" | jq -r '.[0].id'
kill %1
```

## 3. Dry run: what does this person hold?

Without `--apply` the command writes nothing. It lists every client the subject has consented to (with the scopes granted and when) and names the clients it would revoke:

```bash
kubectl -n bex-system exec deploy/bex-api -- /api revoke-oauth-grant <subject> <client-id>
```

`<client-id>` is `429024F5E608930E2A65EF92591A25CC` for the Render CLI (`bex login`), `bex-mobile` for the mobile app, or another listed client. Read the `grants` array of the answer: it is the subject's whole list, so a client you did not expect is visible before you cut anything off.

## 4. Cut it off

```bash
kubectl -n bex-system exec deploy/bex-api -- /api revoke-oauth-grant <subject> <client-id> --apply
# the person's every grant, when you do not know which client holds the stolen token:
kubectl -n bex-system exec deploy/bex-api -- /api revoke-oauth-grant <subject> --all-clients --apply
```

Per client the command does three things, in this order, and answers one JSON line each (`hydra`, `marker`, `audited`):

1. **Hydra.** Deletes the consent session of the `(subject, client)` pair, which revokes every access and refresh token of that grant (`hydra: "deleted"`; `"none"` when Hydra held no session for the pair).
2. **The shared marker.** Advances the `oauth_revocations` row for the same pair. Every replica reads it on every cache hit and evicts a token it cached before that moment: the next request is re-introspected at Hydra, which answers inactive. Hydra goes first on purpose: a marker written before the revocation would let a replica cache "active" newer than the marker.
3. **The audit trail.** One `audit_events` row in the ops workspace's trail (`BEX_OPS_WORKSPACE`, or `default` when unset): verb `oauth.RevokeGrant`, caller `operator` (method `exec`), target `oauth-grant:<client>`, target name the subject. It shows in the workspace audit log (REST and GraphQL) as a `system` actor and is kept for `BEX_AUDIT_RETENTION_DAYS` (90). The pod's log line `bex-api: operator revoked the OAuth grant of subject … to client …` is the second record (Loki).

**The blast radius is the pair.** Revoking `(subject, client)` signs the person out of that client on **every** device (all its tokens share the consent session), and nothing else: their other clients, other people on the same client and every API key keep working. A client holding many sessions of one person is signed out wholesale; the person signs in again, which mints fresh tokens that are newer than the marker. `bex logout` is the single-device alternative and needs the person's own token.

If the command exits non-zero, the answer says which write is missing:

- `the grant is revoked at Hydra but the shared marker was not written`: the token is already dead at Hydra and replicas refuse it within 5 s of their last cache fill. Run the same command again: a repeat is safe (Hydra answers 404 for the session it already removed, which the command accepts, and the marker only advances).
- `the grant is revoked everywhere but its audit row was not written`: the cut-off is complete; write the record by hand in the incident notes and look at why the control-plane database refused the insert.
- a Hydra error: nothing else was written. Check that Hydra answers (`kubectl -n auth get pods`) and run it again.

## 5. Confirm

Ask the person to use the old token (or call the API with it yourself, from a copy you hold): it answers `401` on every replica from the next request on. `kubectl -n bex-system exec deploy/bex-api -- /api revoke-oauth-grant <subject> <client-id>` again lists the subject's grants: the revoked client is gone from `grants`.

## Limits worth knowing

- **Clocks.** The marker is stamped by the database's clock and compared with the time a replica cached the token. A replica whose clock is ahead of the database's by δ keeps a token it cached within δ before the revocation until the 5 s window ends; δ is normally milliseconds. The 5 s bound of ADR012 §8d holds regardless.
- **Hydra has no revocation hook**, which is why the marker exists. A revocation made at Hydra by any other tool (its admin API, RFC 7009 from the client) is still refused within 5 s, just not at the next request.
- **What it does not prove.** It cuts off access from now. It does not tell you what the token did before: read the workspace audit logs of the person's workspaces for the window of the theft.
