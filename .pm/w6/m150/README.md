# w6 · m150 — `render-v1` runs `io_uring` and unprivileged-userns workloads, as on Render (seccomp slice)

**Worker:** worker6 **Goal:** a `render-v1` container can use `io_uring` and `unshare(CLONE_NEWUSER)`, as on Render (no seccomp filter). This is done with a bex-distributed Localhost seccomp profile instead of RuntimeDefault; every other syscall RuntimeDefault blocks stays blocked. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Security review and profile design: RuntimeDefault plus `io_uring` and `CLONE_NEWUSER`; amend ADR089 | 1h | — |
| t002 | Distribute the profile to every tenant node (platform DaemonSet or node bootstrap) | 1h | t001 |
| t003 | Operator: project the Localhost profile onto every `render-v1` path, only where the profile is present | 45m | t002 |
| t004 | Workload admission: admit exactly the bex profile for `render-v1` workloads | 45m | t003 |
| t005 | Live acceptance: `io_uring` and `unshare` probes plus unchanged ClickHouse in a customer workspace | 45m | t004, w6/m149 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

1. **Probe results.** In a customer workspace on production, a new `render-v1` service runs the container-privilege probe (`docs/render-artifacts/fixtures/container-privilege-probe.py`):
   - `io_uring_setup` succeeds;
   - `unshare(CLONE_NEWUSER)` succeeds;
   - `Seccomp` reads `2` (still filtered), and a syscall RuntimeDefault blocks and Render does not need (for example, `kexec_load`) is still refused.
2. **No regressions.** Unchanged ClickHouse still deploys and serves 8123/9000. `strict-v1` and `image-v1` services keep RuntimeDefault, with byte-identical pod templates.
3. **Admission.** Admission refuses any other Localhost profile, `Unconfined`, or the bex profile on a non-`render-v1` workload.
4. **Docs.** ADR089 and ADR018 record the profile, the review and the remaining divergence (a filter still exists; Render has none).

## Source + Goal linkage

- **Source:** w6/085, filed from the w6/m149 re-scope (2026-10-20). Evidence: `docs/render-artifacts/container-privileges.md` (Render `Seccomp: 0`; bex RuntimeDefault blocks `io_uring` and `unshare(CLONE_NEWUSER)`).
- **Goal linkage:** ADR008 § Render parity first. Every image that starts on Render should start on bex unchanged, including Chromium/Puppeteer sandboxes, bubblewrap and `io_uring` servers.
- **Expected outcome:** the last measured runtime-privilege divergence from Render shrinks to "a filter exists" for syscalls no Render-runnable image is known to need.
- **Why now:** w6/m149 shipped `render-v1` on RuntimeDefault. This is the remaining parity gap in the same policy, and the profile must ship before `render-v1` services accumulate, so a later change rolls fewer workloads.
- **Render parity task omitted:** this changes runtime behavior only. No REST/GraphQL/MCP/UI field changes.

## Risks

- Unprivileged user namespaces with fewer syscall restrictions widen the kernel attack surface on shared nodes. t001 must record the tenant-node kernel version, the userns hardening knobs in use, and the residual risk accepted under ADR008.
