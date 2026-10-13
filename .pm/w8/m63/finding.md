# Manual deploy schedules a completed initial hook again until Blueprint re-sync

- **Severity:** major: current API/operator selection replays one-time initialization instead of the intended regular pre-deploy command. No production execution, outage or data loss is claimed.
- **Ownership:** shared backend/operator deployment lifecycle; pinned clients send the correct ordinary deployment request.
- **Versions:** current API/operator source c5a0eb9dd15f6b417bf586fdf26510706497b032; published Bex0.3.2 and unmodified Render2.27/a764810a7682. Live revisions unknown.
- **Context:** functional `$qa-find-bugs-cli` w8 sweep206, 2026-10-13 UTC. Production human QA grant in bex-canary / tea-daif693dqjvc73e7as3g for validation only; manual deployment and operator job creation use current local handlers/fake stores and clients. Non-TTY JSON. Local auth is not certified.

**Evidence boundary:** production accepts the field in both clients, as intended. The replay is proven with actual native processes to current loopback API handlers, then actual current operator job creation into a fake client. First Live rev-1 and predeploy generation1 success are simulated. Hosted Blueprint apply, real job execution and runtime marker counts are untested. Native pinned CLI has no Blueprint apply command and none is invented.

## Reproduction and controls

The local diagnostic creates this declaration through actual public DeployStack:

```yaml
services:
  - name: qa206-hook
    type: web
    runtime: image
    plan: free
    image:
      url: docker.io/mendhak/http-https-echo@sha256:440ca6b810bc04606aac700e461caca5543eaa882c4e0af96a33424d05a23592
    initialDeployHook: echo qa206-initial-once
    preDeployCommand: echo qa206-regular
```

It models the first deploy as Live rev-1, predeploy Succeeded/generation1, without re-syncing the Blueprint. Run actual released Bex and pinned Render against the local server:

```sh
bex deploys create <isolated-local-fixture-id> --confirm -o json
render deploys create <fresh-equivalent-local-fixture-id> --confirm -o json
```

Each process exits0 and sends one actual POST `/v1/services/{id}/deploys`; current handlers open generation2 while keeping spec.preDeployCommand = echo qa206-initial-once and the ran-once marker absent. Actual operator release selection plus predeploy reconciliation produces predeploy-qa206-hook-gen-2 with `sh -c echo qa206-initial-once`. The previous successful step was generation1, so it does not satisfy generation2's regular per-release gate.

Two fresh controls re-sync the same Blueprint after modeled first success, before the native command. Both clients now retain ran=true and echo qa206-regular; the same operator creates generation2 Jobs with the regular command. This isolates the missing completion transition from generic command execution or client transport.

Eight live production validations (both clients × initial-only, initial+regular, empty, omitted) pass; two exact multipart captures confirm correct field admission. [Complete evidence](evidence/cli-sweep206/README.md), [native local requests/results](evidence/cli-sweep206/native-local-results.json), [actual operator Jobs](evidence/cli-sweep206/operator-results.json).

## Root cause, producer and consumer

- apps/service.go:2255–2259 replaces desired.PreDeployCommand with the initial hook on first Blueprint creation; :2291–2298 records the hook text, but not a separate recoverable regular command in this path.
- apps/deploy.go:3132–3145 checks PreDeploySucceeded only when applyCreateToSpec re-syncs the declaration. :3092–3095 writes the ran marker. Production symbol/caller search finds no completion writer in the operator or ordinary deployment path.
- deploys/service.go:762–793 opens the later release and clears historical runtime selection without transitioning completed initial-hook state. It leaves the injected initial command on the saved spec. The actual native wire reaches this same Trigger path; REST:261, GraphQL:225 and MCP:197 delegate to it.
- operator/release_identity.go:417–439 selects the requested later generation; app_controller.go:5568–5600 only treats predeploy success as terminal for the same generation. Its Ensure call at :5652–5673 passes app.Spec.PreDeployCommand directly. A new generation therefore schedules the still-injected initial command.
- Existing stack_test.go:965–986 explicitly performs a second DeployStack before expecting ran=true/regular restoration. All five current hook tests pass, leaving the ordinary native manual-deploy path between first success and re-sync uncovered.

[Exact source excerpts](evidence/cli-sweep206/source.txt). [Render's contract](https://render.com/docs/blueprint-spec#initialdeployhook) distinguishes one-time initialization from preDeployCommand for each deploy. [Completed w2/m45](../../w2/done/m45/README.md) already promises no repetition on subsequent deploys, independently of re-sync. This finding concerns repetition after modeled first Live success. Bex's historical reuse of a pre-deploy Job is retained as the mechanism under review; before-vs-after first-rollout timing is not a separately reproduced finding here and must be recorded accurately during parity review.

## Fix scope and blast radius

Keep the regular command and one-time hook roles separately recoverable, including explicit empty regular command, before first-create injection. Establish completed state at the actual lifecycle boundary instead of waiting for another Blueprint sync. Later release job selection must keep a completed initial hook out while running the regular command, even when their text is identical. Avoid clearing every preDeployCommand or inferring role merely from matching text. Preserve pending/failed hook behavior, source/configuration snapshots and ordinary regular pre-deploy execution. Legacy injected resources without recoverable regular metadata require an explicit, tested migration path; do not invent the original command.

Backend and operator share types as the leaf; no cross-module import or CLI fork is needed. Server-service schema admits web/private/worker hooks; static/cron/KeyValue are not silently added. Only web is modeled here. Audit other release-opening paths, historical restart/rollback selection, Git/deploy-hook flows and re-sync for the same completion state; those sibling paths are untested. Preserve existing request/response envelopes and saved-field readbacks. No new one-off-job product is proposed.

## Dedupe, tests and cleanup

Open/blocked/done searches found w2/done/m45 and its source w2/done/013, but no manual-deploy-before-re-sync fix. This is a demonstrated gap in that completed DoD, not a new initial-hook feature. Whole prior DoD: initial creation and re-sync markers/regular restoration are locally modeled and covered by five existing tests; native later manual-deploy selection now fails without re-sync. Hosted first rollout/hook execution, preview instances and field echo across all surfaces were not replayed and remain acceptance obligations. No unrelated w2 board status is changed; schedule the cross-module fix in w8.

Four current local native cases and four actual operator diagnostics reproduce two incorrect selections and two correct re-sync controls. Assertions expect current wrong selection and are not fixed-source regressions. No full backend/operator suite is claimed. Three excluded operator fixture attempts omitted fake API metadata generation, preceding release selection and server UID; final probes model those server effects and call actual release selection. Long Go JSON event chunk decoding was corrected separately. No product claim comes from those QA setup failures.

No production resource or paid feature is created. All local native processes, httptest/forwarding servers and temporary configs close/remove; real quota remains baseline6/termination0 and no datastores. Local fake IDs in receipts are not production ownership proof or deletion targets. Isolated human session stays active for the requested loop. No implementation, commit, push, release or external submission. [Metadata](evidence/cli-sweep206/metadata-cleanup.json).

## Source identity follow-up — sweep207

The recorded c5a0eb9dd15f base and verified 3c6809182 head have byte-identical apps/deploys/core/controller/types/CLI source trees. Exact HEAD during every earlier diagnostic was not separately captured; this [tree-identity record](evidence/cli-sweep206/source-identity-followup207.json) narrows provenance accurately. New commits changed metrics/harness and desired images, rather than the diagnosed hook paths. No deployed revision is inferred and no source fix is claimed.
