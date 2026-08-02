# Agent image credential contract

This directory currently supplies ADR047 D2's credential-enabled image base. The adjacent w3/m37 milestone adds the session driver, language toolchains, and ACP agent binaries on top of it.

Git is configured system-wide with:

```text
credential.helper=bex
credential.useHttpPath=true
```

`git-credential-bex get` calls the gateway for every Git operation and emits the one-hour GitHub installation token only over stdout to Git. `store` and `erase` are no-ops. There is no cache or token file.

The session-create path must stamp the non-secret labels returned by `agentsession.BindingLabels` and set these non-secret environment values:

- `BEX_SANDBOX_NAMESPACE`
- `BEX_AGENT_SESSION_ID`
- `BEX_AGENT_REPOSITORY`
- `BEX_AGENT_BRANCH` (`bex-agent/*`)
- `BEX_AGENT_CREDENTIAL_URL` (defaults to the internal gateway Service)

Immediately before an OpenSandbox rootfs snapshot, run `/usr/local/bin/bex-pre-snapshot` and abort the snapshot if it fails. It clears only enumerated credential locations and any credential-cache daemon; it never scans or deletes tenant source files based on token-like content. On resume, Git calls the helper again, so no restoration step or cached credential is required.

`credential-e2e-test.sh` builds and runs the real image against a hermetic GitHub-compatible smart-HTTP origin. It performs clone/fetch/push, advances the fixture clock beyond GitHub's one-hour token TTL, commits the stopped container rootfs, resumes from that image, and pushes again. Both the original and resumed rootfs are scanned for the fixture token prefix. This is the locally exported rootfs/dev-substrate evidence allowed by w3/m38 t004 while OpenSandbox's secure production snapshot path remains owned by w3/m42.
