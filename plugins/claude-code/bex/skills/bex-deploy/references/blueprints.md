<!-- shared:blueprints -->
## bex Blueprints

A bex Blueprint is a declarative YAML definition of interconnected bex services, databases, and environment groups. The file is usually named `render.yaml` and stored at the root of a Git repository, although its path can be customized when the Blueprint is created.

Blueprint creation and sync read the file from the linked remote Git repository. Local edits are not available to bex until they are committed and pushed to that repository.

### Author and validate

Do not rely on remembered Blueprint fields, allowed values, defaults, or update behavior. Before creating or modifying a Blueprint, use the documentation-retrieval workflow in the root `SKILL.md` to retrieve and read the [Markdown reference](https://render.com/docs/blueprint-spec.md) or its [HTML version](https://render.com/docs/blueprint-spec).

If neither version can be retrieved, use the current JSON Schema and bex validation where possible. Use the bundled guidance only for stable workflow and safety constraints, and do not guess at unfamiliar or potentially changed syntax.

Read the sections relevant to every resource and field being changed. In particular, confirm required fields, service-type support, immutable fields, defaults for new resources, behavior for existing resources, and whether omitting or emptying a field preserves or removes existing configuration. Do not treat an example Blueprint as a complete specification.

Use the public JSON Schema for editor or programmatic validation:

```text
https://render.com/schema/render.yaml.json
```

When the bex CLI is available, validate the finished file:

```bash
bex blueprints validate render.yaml
```

Fix validation errors before presenting the Blueprint as ready to deploy. Validation does not apply, sync, or deploy the Blueprint. Schema validation checks structure; bex CLI or API validation can also catch platform-aware errors. If copied examples, the schema, and bex validation disagree, prefer the configuration accepted by current bex validation and call out the discrepancy.

Blueprint resource names are used to associate definitions with existing resources. Before reusing or changing a name, verify that the Blueprint will manage the intended resource. Do not define the same resource in more than one location, such as both a root-level list and a project environment or `ungrouped`.

Treat edits to an existing Blueprint as infrastructure changes, not ordinary text edits. Review the proposed diff for replacements, removals, immutable-field changes, and destructive declarative behavior before applying or recommending a sync. Never hardcode secret values in a Blueprint; consult the current environment-variable sections of the specification for supported secret mechanisms and their limitations.

### Discover deployment capabilities

Blueprint operations are in public alpha. Inspect the active MCP tool schemas or installed CLI help before using them; do not assume every client exposes them. The CLI offers create, plan, approve, and update under `bex ea blueprints`. Local validation remains `bex blueprints validate`.

| Operation | MCP | CLI |
|-----------|-----|-----|
| Create and plan the first sync | `create_blueprint` | `bex ea blueprints create` |
| Plan an existing Blueprint | `plan_blueprint` | `bex ea blueprints plan <blueprintId>` |
| Apply its newest planned sync | `approve_blueprint` | `bex ea blueprints approve <blueprintId>` |
| Update settings | `update_blueprint` | `bex ea blueprints update <blueprintId>` |
| Read settings, status, and managed resource IDs | `get_blueprint` | Inspect the current command surface; do not invent a Blueprint get command |

Prefer available MCP tools, then a capable CLI. If neither exposes the required operation, use the Dashboard or current REST API. For exact contracts, inspect the current MCP tool definitions (`tools/list` on https://api.bex.co/mcp) and [CLI reference](https://bex.co/docs/cli). Separate local schema/CLI/API validation from remote planning; MCP has no dedicated Blueprint validation tool.

### Prepare the remote source and resource scope

Confirm the workspace and reuse its `workspaceId` on MCP calls that support it. Reuse the existing Blueprint ID (`exs-...`) when updating its infrastructure. A sync ID is not a Blueprint ID.

Before creating or planning, verify that the intended YAML revision is pushed to the configured branch and repository-relative path. Merging into the default branch is not required. Preserve unrelated checkout changes and publish only within the user's authorization. Before pushing to an existing Blueprint's branch, check whether auto-sync will apply that push immediately.

For MCP creation, supply `repoUrl`, the intended `branch`, and `path` if different from `render.yaml`. Normalize an SCP-style remote such as `git@github.com:org/repo.git` to `https://github.com/org/repo`; the tool accepts HTTPS or `ssh://` URLs, not SCP syntax. Pass the branch separately from the URL.

CLI creation without `--repo` uses local `origin` and the current branch. With `--repo`, it defaults to the remote's default branch; pass `--branch` to use another branch. `--path` is relative to the repository root, not the current directory. After verifying the intended workspace and push, an example is:

```bash
bex ea blueprints create --repo https://github.com/acme/app --branch release --path infra/render.yaml --auto-sync=false --output json
```

Choose existing-resource and auto-sync behavior deliberately:

- `existingResources: create_new` / `--existing-resources create_new` is the default. It creates separate resources with a Blueprint-specific suffix, leaving existing resources unchanged. Repeated creation can produce unwanted duplicate infrastructure; keep the returned Blueprint ID.
- `adopt` brings matching existing resources under management and updates their configuration. Use it only when the user intends to manage those resources, after checking resource identity and ownership. Review any ownership-conflict notices before approval.
- `autoSync` / `--auto-sync` defaults to true. After first approval, later Git pushes automatically apply changes. Explain this before approval; use false when the user wants manual review of future changes. Manually requested plans always need approval, even with auto-sync enabled.

### Interpret the plan before approving

Creation saves the Blueprint and plans its first sync; it does not apply resource changes. Do not immediately request another plan after successful creation. Planning an existing Blueprint reads its configured remote branch/path. Wait for a pending or running sync to finish before requesting another plan.

Read `planningOutcome`, `sync`, and `notices`. CLI JSON wraps these in `data` and includes guidance in `meta`; MCP returns the result fields directly.

| Outcome | Next action |
|---------|-------------|
| `planned` | Review the returned proposed changes, notices, and `sync.requiredManualEnvVars`; share `sync.dashboardUrl` when present. Apply only after the user authorizes the reviewed plan. |
| `no_changes` | Report that this attempt found no infrastructure changes. Do not approve. |
| `failed` | Inspect notices/errors, fix the cause, push corrected YAML if needed, then re-plan the same Blueprint. Do not approve, even if a sync object exists. |
| Missing or unknown | Inspect the current contract and returned error/state. Do not infer success or approve. |

Approval targets the Blueprint's newest saved sync, not a caller-specified sync ID. bex's `approve_blueprint` also accepts the reviewed plan's `sync.id` as an optional `syncId` (a bex extension): pass it, and bex refuses the approval when a newer plan has replaced the one the user reviewed. A failed attempt or a no-change result can leave an older plan awaiting approval. Never run approval as an unconditional next command. If another actor has planned changes since review, or the plan expired, obtain and review a fresh plan before applying. Unapproved plans expire after 24 hours under the current contract.

If a create/update request fails after potentially saving state, inspect the Blueprint before retrying. Do not recreate it merely because planning failed. Report saved settings and planning success separately.

### Supply manual values and apply

Use the entries in `sync.requiredManualEnvVars` as the checklist. For MCP, supply `manualEnvVars` entries with the exact `serviceType`, `serviceName`, `key`, and user-provided `value`. Reuse values already supplied; ask only for missing ones and never invent credentials. Do not print secrets or put them in the Blueprint or repository. Values sent through MCP appear in the agent context.

CLI create/plan writes a JSON array of those entries with empty value placeholders to the returned manual-env-vars file. Fill required values before approval: an empty placeholder is submitted as an empty string, not treated as missing. A newly planned sync replaces the generated file, or removes it if no manual values are required, so check it again after re-planning.

Use `bex ea blueprints approve <blueprintId> --output json` for inspectable approval output, substituting the actual Blueprint ID. It reads the generated file by default and deletes it after approval. `--manual-env-vars-file <path>` reads a custom JSON file (not `.env`) and leaves it on disk; `--manual-env-vars-file -` reads stdin. Keep any such file outside the repository with restricted permissions and remove it when no longer needed.

If the user wants values kept outside agent context, have them supply values directly through CLI or the approval API. Alternatively, they can approve at the returned Dashboard URL and then set values on each service's Environment page; explain that initial deployments may fail until those values are set.

Call approve only after the user authorizes applying the reviewed plan. Reuse authorization that already covers that plan; providing values, allowing secret generation, or accepting costs alone does not authorize deployment. The call returns before the sync and service deployments finish.

### Update settings and verify the result

Update accepts name, branch, path, and auto-sync settings; it does not change the repository. Omitted settings are retained. Branch/path changes also request planning; name/auto-sync-only changes do not. MCP skips the update and planning if all supplied values already match. Settings take effect immediately and can remain changed even when planning fails. An MCP response with a planning-stage `error` means the settings were updated but no planning result is available. Inspect state before retrying an uncertain update.

Use `get_blueprint` when available to read settings and managed resource IDs. Do not equate Blueprint status with workload health: `created` persists until first approval; after approval, `paused` describes disabled auto-sync even while manual syncs are possible; `in_sync` can also mean a plan awaits approval. Follow the resulting resource deployments to terminal states, inspect failures, and verify application behavior before reporting success. Share the returned Dashboard URL and distinguish planning, approval, sync completion, and workload readiness.
<!-- /shared:blueprints -->
