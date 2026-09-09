# Environment-group metadata CAS — 2026-09-08

Proves the w4/m97 contract: overlapping content-save, rename, link, unlink, delete, and stale-link prune requests against one durable store cannot restore a stale name, Environment, or link set, and a deleted group cannot be resurrected by a delayed writer.

## Failure mode (reproduced in Core)

Before the fix, `touch` / `writeMeta` replaced the entire metadata map from a caller-held snapshot. A content patch that captured meta before claiming the content revision, then called `touch` after writes, restored an older name or link set if another request had already committed. `abortGroupPatch` unconditionally rewrote `oldMeta`. `pruneStaleLinks` mutated the pre-rollout snapshot. Concurrent `linkFetched` calls each patched their App successfully, then last-writer-wins metadata dropped one link.

## Fix

`mutateMetaCAS` dual-reads the workspace (and legacy) metadata map, applies a field-local mutation, and `PutCAS`es it with bounded retries. `touch` only bumps `updatedAt` on the current map. Rename claims the new name, CAS-commits, then releases the old claim. Move revalidates current links each attempt. Link/unlink CAS-merge membership; link compensates the App when meta returns `ErrNotFound`. Delete CAS-clears meta, detaches the captured links, and plants a non-editable tombstone after artifact removal. Content compensation no longer restores metadata.

Public additive signal: `ENV_GROUP_METADATA_CONFLICT` (409). Existing `save_only`/`deploy`/`rebuild` and names-only reads are unchanged.

## Automated evidence

From `lego/backend`:

```bash
go test ./internal/envgroups/ -count=1 -race -timeout 120s
```

New regressions in `meta_cas_test.go` fail against unconditional metadata writes and pass with CAS (rename-vs-content, compensation-vs-rename, dual-link, prune-vs-link, delete-vs-delayed-writer, dual-rename, hard conflict). Existing create/patch/link suites remain green.

## Live drill (dev-4)

Intended exercise against the workstream's isolated stack (`bash scripts/dev-env.sh 4 status` healthy, real OpenBao + Kubernetes):

1. Create one env group and two compatible services under the same Environment.
2. Pause a content patch mid-revision (or use two bex-api replicas) while renaming; confirm the committed name and content both survive.
3. Concurrently link each service; confirm both Secret refs and both memberships.
4. Delete the group; retry a stale link/content writer; confirm `404`/`ErrNotFound` and no resurrected meta.
5. Delete scratch services, group, and any Environments/Projects created for the drill. Do not record secret values.

**Session limitation:** the controlled Core tests above are the dated proof for this ship. A live OpenBao/Kubernetes pass on `dev-4` was not executed in this session (no healthy mock-cluster kubeconfig available here); re-run the steps above on the next worker session that has `dev-4` up and record fixture ids + HTTP classes in this file without secret material.

## Scope boundary

Interrupted-operation recovery (stranded busy revision / crash mid-rename) remains `w4/m98`. This milestone does not reset busy markers to conceal a stranded content operation.
