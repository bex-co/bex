# ADR035 — SSH into running service instances

**Status:** accepted; implemented behind production activation and live verification (2026-07-14)

## Context

Render supports public-key SSH into paid web services, private services, and background workers. Its current CLI reads `serviceDetails.sshAddress`, verifies a live deploy, optionally lists `GET /v1/services/{id}/instances`, and invokes the user's local OpenSSH binary. This is narrower than an ephemeral shell product: it attaches to one container that is already serving the App.

bex previously grouped every exec-shaped feature under the hosted-execution non-goal. The user reopened only running-instance SSH. Ephemeral instances, one-off jobs, browser terminals, and sandboxes remain excluded in [`.pm/DO_NOT_DO.md`](../.pm/DO_NOT_DO.md).

The security constraint is stronger than ordinary API reads: Kubernetes `pods/exec` can read the complete runtime environment and act with the workload's identity. bex-api must not inherit that permission.

## Decision

bex runs `/ssh-gateway` as a third backend entrypoint in the shared lego image and as its own Deployment and ServiceAccount. It accepts SSH on container port 2222. Traefik's dedicated `ssh` TCP entrypoint exposes it on public port 22 in production.

The connection path is:

1. OpenSSH connects to `<service-id>@ssh.bex.co`, or `<instance-id>@ssh.bex.co` for a selected replica.
2. The gateway accepts public-key authentication only. It hashes the presented key with OpenSSH SHA-256 and resolves that globally unique fingerprint in `ssh_keys`.
3. After the client proves key possession, the gateway attaches the stored subject as a `core.Identity` with method `ssh`.
4. `apps.Service.ResolveSSHSession` parses the username, resolves the App through its stable `srv-…` id, and calls the resource-scoped `AuthorizeApp(can_operate)` seam. Authorization therefore targets the App's workspace, not whichever workspace happens to be the caller's default.
5. The resolver selects a Running, Ready pod whose revision label and `app` container image equal the App's active status. A bare service id selects a random eligible replica, matching Render. A specific instance id selects exactly the pod returned by the instances endpoint.
6. One SSH `session` channel maps to one Kubernetes `pods/exec` stream in the existing `app` container. No sshd or sidecar is injected into tenant images.

## Public contract

### SSH keys

Keys are identity-scoped. One person can use the same key in every workspace where that identity has `can_operate`; a key is not copied into workspace membership or Kubernetes Secrets.

- REST: `GET/POST /v1/ssh-keys`, `DELETE /v1/ssh-keys/{id}`
- GraphQL: `sshKeys`, `createSSHKey`, `deleteSSHKey`
- MCP: `list_ssh_keys`, `add_ssh_key`, `delete_ssh_key`
- Dashboard: Account Settings → SSH Public Keys

Eligible service pages expose the command through a Connect → SSH menu, matching Render's documented dashboard flow; unavailable services explain the missing address without inventing one.

The store contains typed `ssk-…` id, subject, display name, canonical public key, SHA-256 fingerprint, and creation time. It never accepts or stores private material. Supported key types match Render's documented set: Ed25519, RSA (minimum 2048 bits, authenticated only with SHA-2 signatures), ECDSA P-256/P-384/P-521, and OpenSSH security-key Ed25519/ECDSA. Comments are discarded. Multiple records, `authorized_keys` options, trailing payloads, oversized input, malformed text, and duplicate key material are rejected. Options are rejected rather than silently stripped because a user might otherwise believe an option such as `command=` still restricts the registered account key.

Render documents RSA support but no minimum size. The 2048-bit registration floor is bex's explicit security policy; it rejects obsolete weak RSA material instead of advertising a key that the gateway should not trust.

Any workspace member may manage their own public keys through `can_manage_ssh_keys`; only contributor/developer/admin identities can open sessions because target authorization separately requires `can_operate`. Deleting a key prevents every new handshake using it. Existing sessions are not retroactively killed by key deletion.

### Service and instance shape

When `BEX_SSH_HOST` is set and the SSH-key store is available, an eligible service returns:

```json
{
  "serviceDetails": {
    "sshAddress": "srv-d5t5d4v8g3c73f5m9peg@ssh.bex.co"
  }
}
```

The field is omitted for free, suspended, cron, static, or unsupported service types and when no public host is configured. Web, private, and background-worker services on a paid plan are eligible to advertise an address; the gateway still requires Running phase, an active revision, and a Ready current-image pod before authenticating.

`GET /v1/services/{id}/instances` returns Render's array of `{id,createdAt}` objects. Public instance ids are `<service-id>-<opaque-suffix>`, stably derived from the Pod UID; raw pod names, UIDs, or Kubernetes credentials are never exposed. The general instances surface follows Render's rollout-observability contract: it includes non-terminating Pending, Running, and Unknown Deployment pods and excludes terminating, terminal, cron, and static pods. The SSH resolver then re-lists the service pods and narrows a requested id to a Running+Ready pod whose `app.bex.co/revision` label equals `App.status.activeRevision` and whose app-container image equals `App.status.image`. Build, pre-deploy, stale-revision, stale-image, and non-Ready pods therefore remain visible where appropriate for instance inspection but fail closed as SSH targets. The operator stamps the revision label on every app Deployment pod template, so a same-image restart cannot make the old ReplicaSet look current.

## SSH protocol and exec behavior

The server permits only:

- one `session` channel per SSH connection;
- `pty-req` before execution;
- `window-change` while a PTY session runs;
- one `shell` request, executed as `/bin/sh`; or
- one `exec` request, executed as `/bin/sh -lc <client-command>`.

The command is one argv value, not interpolated into a Kubernetes URL or host command. It intentionally receives normal shell semantics inside the app container. The gateway does not log or persist it.

stdin, stdout, stderr, TTY state, and resize events bridge through client-go `remotecommand`. Kubernetes exit codes become SSH `exit-status`. Client disconnect, gateway shutdown, session timeout, restart, redeploy, or pod deletion cancels the stream. An image without executable `/bin/sh` fails with exit 126 and a bounded shell-unavailable message; bex never installs a shell into the image.

The following never reach Kubernetes: `direct-tcpip`, forwarding, agent forwarding, X11, subsystems including SFTP, SCP protocol handling, environment requests, and extra session channels.

## Isolation and RBAC

The gateway ServiceAccount has one Role in the configured tenant App namespace: `get,list` on `apps.app.bex.co` and `pods`, plus `create` on `pods/exec`. It has no ClusterRole or ClusterRoleBinding.

It cannot read Secrets, logs, Jobs, platform/auth namespaces, or arbitrary tenant namespaces. bex-api's separate Role remains read-only for pods and has no `pods/exec` rule. A structural manifest test guards this split and rejects any future cluster-wide gateway grant.

The gateway runs non-root with all capabilities dropped, a read-only root filesystem, resource limits, two replicas, a PodDisruptionBudget, and SSH ingress restricted to the Traefik namespace. A separate internal HTTP listener supplies health probes and Prometheus metrics; only the monitoring namespace may reach it. Metrics contain bounded authentication/session outcome and limit-scope labels—never subjects, service or instance ids, addresses, commands, environment values, or terminal content. Global and per-identity session caps default to 100 and 5. Handshakes default to 10 seconds and sessions to four hours.

## Host-key custody and network exposure

The gateway host key is unrelated to the `~/.ssh/bex` node-admin key described in [ADR019](ADR019-infra-credentials.md). It is a dedicated, stable Ed25519 key installed out of band as `bex-system/bex-ssh-host-key` by [`scripts/ssh-host-key-secret.sh`](../scripts/ssh-host-key-secret.sh). The Deployment refuses to start when the Secret is absent or malformed.

The local hostNetwork Traefik baseline binds its SSH entrypoint on 2222, avoiding node sshd on port 22. The production Hetzner LoadBalancer exposes that entrypoint on 22. The `ssh.bex.co` A/AAAA records must be DNS-only and point directly at the Traefik LoadBalancer; an ordinary Cloudflare-proxied record cannot carry this raw SSH endpoint. [`scripts/ssh-dns-cloudflare.sh`](../scripts/ssh-dns-cloudflare.sh) reconciles that exact DNS-only A/AAAA set and removes stale or duplicate address records. Its Cloudflare token needs zone-specific `Zone / DNS / Edit`; zone discovery additionally needs `Zone / Zone / Read`, which can be avoided by supplying `CLOUDFLARE_ZONE_ID` (the existing `CF_ZONE_ID` name is also accepted). The token is an ephemeral operator credential and must not be written to `.env` or a checked-in file. `--check` is read-only and rejects proxied, missing, stale, or duplicate address records.

[`scripts/ssh-activate.sh`](../scripts/ssh-activate.sh) `--check` is a separate read-only preflight that requires the complete public A/AAAA set to equal the Terraform-owned Hetzner edge's public address set and verifies TCP/22 presents the mounted key's published fingerprint. Both SSH edge scripts query that exact named object with `HCLOUD_TOKEN`; they no longer depend on Kubernetes `LoadBalancer` status because the production Traefik Service is intentionally a `NodePort`. Running activation without `--check` performs the same gates before creating the activation ConfigMap and restarting bex-api. `BEX_SSH_HOST` must not be set before those checks pass.

Rotation is deliberate: create a new key, publish its fingerprint and maintenance window, replace the Secret with `scripts/ssh-host-key-secret.sh` (which rolls and waits for both gateway replicas), and tell clients to replace the known-host entry. Routine deploys never regenerate the key.

## Audit contract

`AuthorizeApp(can_operate)` records the allowed or denied connection authorization through the shared audit seam. A successful handshake additionally creates an `ssn-…` row in `ssh_sessions`, then closes it with end time and result. The table contains only subject, workspace, service id, instance id, remote address, start/end times, and result. The daily audit sweep deletes these rows after `BEX_AUDIT_RETENTION_DAYS` (default 90), the same boundary as `audit_events`.

The audit type and schema have no command, argv, environment, stdin, stdout, stderr, or terminal-content field. This structural omission is the privacy boundary; operators must not add ad hoc stream logging around it.

## Fail-closed behavior

Authentication fails without revealing whether the cause was an unknown/deleted key, malformed username, missing/foreign service, insufficient role, unsupported/free/suspended service, missing live revision, or missing/non-Ready/stale instance. No session channel opens and no `pods/exec` request occurs.

After authorization, Kubernetes errors remain bounded. A pod disappearing during restart/redeploy closes the stream. The gateway never retries against another replica, because silently switching a specific session target would violate instance selection and could cross a deploy boundary.

## Threat model

- **Cross-workspace target guessing:** stable service ids resolve through `AuthorizeApp`; the resource's tenant label determines the OpenFGA object.
- **Key ambiguity:** fingerprint uniqueness permits exactly one subject per public key.
- **Deleted key races:** every new handshake reads the database before accepting the offered key and rechecks it after signature verification, before target authorization; no key cache survives deletion.
- **Pod-name confusion:** clients receive a derived instance id, and resolution re-lists label-selected Ready pods rather than accepting a raw pod name.
- **Stale rollout target:** pod container image must match observed active image and the pod must be Ready and non-terminating.
- **Command injection into gateway/Kubernetes:** the SSH command is passed as one `/bin/sh -lc` argv element; it is never evaluated by the gateway host or embedded into a URL.
- **Session exhaustion:** bounded handshake/session durations and global/per-identity counters reject excess connections.
- **Compromised gateway:** namespaced pod/exec RBAC limits the Kubernetes blast radius to the configured App namespace; it cannot read platform Secrets. This remains a powerful tenant-workload privilege, which is why the binary and ServiceAccount are isolated. The initial deployment still uses the control-plane application's database credential for key lookup and audit writes; replacing it with a separately granted lookup/insert/update-only role is a defense-in-depth follow-up because the narrow Go `Store` interface does not constrain a stolen database credential.
- **Host-key substitution:** stable out-of-band custody and published fingerprints make unexpected replacement visible to OpenSSH.

## Explicit non-goals

This ADR does not authorize ephemeral shell instances, `--ephemeral`, one-off jobs, cron shell access, browser terminals, hosted sandboxes, SFTP/SCP, forwarding, agent forwarding, direct TCP/Unix-socket channels, shell installation, or an sshd sidecar.

## Verification

Deterministic tests cover parsing, canonical fingerprints, duplicate/foreign ownership, REST/GraphQL/MCP parity, service eligibility, stale-revision/non-Ready filtering, any/specific-instance resolution, a real in-process SSH handshake, PTY resize, exit status, deleted-key and forbidden-target denial, forwarding rejection, session caps, authenticated-idle timeout/release, and content-free audit writes.

An opt-in real-cluster test (`TestGatewayRealKubernetesExec`) covers the boundary those fakes cannot: SSH protocol → client-go SPDY → a disposable pod's `app` container. Against the local CAPD app cluster on 2026-07-15 it read a runtime environment value, propagated exit 37, and closed the attached stream when pod deletion completed its 30-second termination grace. Its shell-less sibling drove a real `traefik/whoami` container and received the bounded exit-126 response. The tests require an explicit kubeconfig and disposable pod names; the deletion check additionally requires `BEX_TEST_SSH_DELETE_POD=1`, so ordinary `go test ./...` never mutates a cluster.

Production activation additionally requires [`scripts/ssh-verify.sh`](../scripts/ssh-verify.sh) to pass against public TCP/22 with raw OpenSSH and the current unmodified Render CLI. Until that evidence exists, parity documents must describe the implementation as awaiting live verification rather than claiming a production pass.

The production sequence is intentionally fail-closed: deploy the image and manifests; install the dedicated stable host-key Secret; expose Traefik's `ssh` entrypoint; reconcile and check the direct, DNS-only A/AAAA records with `scripts/ssh-dns-cloudflare.sh`; verify the public fingerprint; and only then run `scripts/ssh-activate.sh` to advertise `sshAddress`. If any earlier step is absent, bex-api continues omitting the address.

The verifier requires the API URL/token, a disposable private-key file, and the published host-key fingerprint. By default it creates and cleans up a paid, shell-capable two-replica service; an explicitly supplied existing service must be disposable because the matrix restarts, suspends, resizes, and temporarily rolls it to a shell-less image. It registers only the derived public key, pins the observed TCP/22 host key, checks runtime environment, PTY resize, any- and specific-instance raw SSH, exit status, stale-instance denial, separate restart- and redeploy-driven stream closure, suspended/free/shell-less behavior, and unknown/deleted keys. `BEX_RENDER_CLI_VERIFY=1` adds interactive CLI-by-service-name plus direct-instance-id checks; `BEX_RENDER_CLI_BIN` can pin an exact downloaded release binary instead of relying on `PATH`. The script supplies `RENDER_HOST`, `RENDER_API_KEY`, and `RENDER_WORKSPACE` to the unmodified binary without persisting their values, pins the OpenSSH host key, and asserts the destination the CLI selected. The current official release, `render-oss/cli` v2.21.0 at `c398207`, still has an upstream defect in both instance-picker callbacks (they drop the selected instance id), so exact-replica acceptance uses the CLI's supported full-instance-id argument instead of claiming that broken menu path works. `BEX_SSH_VERIFY_FULL_MATRIX=1` additionally requires out-of-band viewer, foreign-workspace, static, and cron fixtures, because the primary acceptance identity must not be able to manufacture its own weaker role or foreign workspace.
