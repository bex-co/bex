# w2/m39 production SSH acceptance — 2026-07-17

This is the sanitized closeout record for public running-instance SSH. It contains no private key, bearer token, OAuth client secret, kubeconfig, tenant environment value, captured terminal stream, or command output.

## Activated production edge

- `ssh.bex.co` resolved directly to DNS-only `A 49.12.20.236` and `AAAA 2a01:4f8:c01e:3d1f::1` records.
- Public TCP/22 presented the installed stable Ed25519 fingerprint `SHA256:cwBcvu8ou7s53NcHrFMxqd3955BuoJRhDx9mMMGFpNE`.
- `bex-system/bex-ssh-gateway` was 2/2 Ready and `bex-system/bex-ssh` advertised `ssh.bex.co` to bex-api.
- `https://api.bex.co/healthz` returned 200. The production dashboard Shell route returned the expected authenticated-route redirect to `/auth/login?next=%2Fservices%2Feden-cms-v2%2Fshell`, proving the shipped route is active without exposing it anonymously.

The production authorization audit returned the intended matrix:

- gateway App/pod read and `pods/exec create` in the tenant namespace — allowed;
- gateway Secret read in the tenant namespace — denied;
- gateway `pods/exec create` in `bex-system` — denied;
- bex-api `pods/exec create` in the tenant namespace — denied.

Existing HTTP ingress remained healthy. The SSH route terminates at the isolated gateway and did not alter the node-admin SSH trust path.

## One uninterrupted public-edge matrix

`scripts/ssh-verify.sh` completed through `ssh.bex.co` using a disposable Scale workspace, a paid two-replica service, separate primary/viewer Ed25519 keys, a foreign workspace service, and static/cron controls. The successful fixture used service `srv-d9dh3vc5btjc73dr9vi0`, exact instance `srv-d9dh3vc5btjc73dr9vi0-anv9c71ccqjjglqlo1kr`, and disposable key record `ssk-d9dh46bjvgns73b7tsv0`.

Passing checks:

- stable public host fingerprint;
- unknown key rejection before registration;
- raw OpenSSH any-instance runtime attachment;
- raw exact-instance selection and exit-status propagation;
- interactive PTY allocation, runtime environment, and terminal resize;
- official Render CLI by service name;
- official Render CLI by complete instance id with runtime validation;
- restart closed the attached session;
- the replaced/stale instance id was rejected;
- suspended and free services were rejected, with `sshAddress` omitted for free;
- a real image redeploy closed the attached session;
- a shell-less image returned the bounded exit-126 contract;
- unknown service, viewer, foreign-workspace service, static site, and cron job were rejected;
- deleting the registered key rejected the next connection;
- the disposable service was deleted.

The runner then deleted its three workspaces and OAuth clients. A final production audit found zero `m39`/`ssh-verify` App fixtures and zero `m39-` OAuth clients. One orphan from an earlier diagnostic attempt was separately removed through the product API with a temporary exact-workspace authorization grant; the grant was revoked.

## Official Render comparison

- The unmodified checksum-verified official CLI v2.21.0 (`c398207`) reported `render v2.21.0` and passed both working public arguments: service name and complete instance id.
- v2.21.0's two interactive instance-picker callbacks still discard the selected `instanceID` and pass the service id. This is an upstream CLI defect; the acceptance run did not patch it or claim that menu path.
- The CLI's direct-id ownership check reads its persisted CLI config even when list requests honor `RENDER_WORKSPACE`. The verifier therefore sets an isolated `RENDER_CLI_CONFIG_PATH`, preventing an unrelated Render workspace from contaminating a bex-only acceptance context.
- Render's current public OpenAPI exposes `serviceDetails.sshAddress` and `GET /services/{serviceId}/instances` as an array of required `{id, createdAt}` objects. It exposes no SSH-public-key management endpoints; bex's REST/GraphQL/MCP key surfaces remain an intentional superset.
- Render's current SSH documentation supports browser Shell plus terminal SSH for paid web/private/background-worker services, random or exact running-instance targeting, and automatic closure on restart/redeploy. Those running-instance behaviors passed above.

## Explicit remaining divergences

- bex's dashboard Shell destination is a copy-ready OpenSSH guide, not Render's browser-hosted terminal; private keys and terminal streams never enter the browser.
- Ephemeral shells (`render ssh --ephemeral`), Render's temporary cron Shell, SFTP/SCP, agent forwarding, and TCP/Unix-socket forwarding remain excluded.
- Minimal images without `/bin/sh` remain unsupported and fail with the bounded exit-126 response; bex does not modify tenant images or inject sshd.
