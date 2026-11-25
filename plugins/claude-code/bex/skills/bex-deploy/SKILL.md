---
name: bex-deploy
description: Deploys applications to bex by analyzing codebases, authoring Blueprints, reviewing infrastructure plans, and applying authorized changes through MCP or CLI. Use when the user wants to deploy, host, publish, or set up an application on bex, or plan and approve an existing Blueprint's changes.
license: MIT
compatibility: Blueprint deployments require a pushed Git repository. MCP Blueprint tools or a CLI exposing bex ea blueprints are required for programmatic deployment; Dashboard is a fallback. Direct image-backed deployments can use the CLI without a Git repository.
metadata:
  author: bex
  version: "1.2.0"
  category: deployment
---

# Deploy to bex

Carry the requested deployment through configuration, plan review, authorized application, and workload verification. If the user asks only for YAML or a plan, stop at that scope. Ask only for missing information; reuse the source, workspace, resources, and authorization already established. When asked for commands only, use the supplied resource IDs instead of generic placeholders and honor any instruction not to execute them.

## Choose the deployment path

Analyze the repository using [references/codebase-analysis.md](references/codebase-analysis.md). Identify the runtime, build/start commands, required variables, datastores, workers, cron jobs, and port binding before choosing a path.

| Path | Use when |
|------|----------|
| Blueprint | An existing Blueprint manages the app; multiple services or datastores are needed; shared configuration, monorepos, or reproducible IaC matter |
| Direct creation | A single web service or static site needs no separate infrastructure, or the user explicitly wants an individual resource |

Prefer MCP when the required tools are available. Use the CLI when MCP is unavailable or lacks an operation; do not require MCP setup when the CLI can complete the task. Use the Dashboard or API for remaining gaps or when requested.

Git-backed services need pushed code accessible to bex. Blueprints always read a YAML file from a remote Git repository, including Blueprints with `runtime: image`. Direct prebuilt-image deployment can use CLI `bex services create --image` without a Git repository; inspect its help for required options. The MCP direct service tools do not accept prebuilt images, but MCP can create a Blueprint that references one.

## Establish access and scope

Read [references/bex-access.md](references/bex-access.md) before accessing bex. Verify the intended workspace and current tool capabilities; pass the confirmed `workspaceId` on MCP resource calls that support it. Reuse existing access and ask about workspace identity only if unresolved.

Before changing resources, establish which existing Blueprint or services belong to this deployment. Do not create another Blueprint merely because an earlier planning attempt failed. Explain the intended resources and applicable costs using [references/compute-plans.md](references/compute-plans.md).

## Blueprint deployment

Read [references/blueprints.md](references/blueprints.md) before authoring, creating, planning, updating, or approving a Blueprint. It defines the shared lifecycle, capability checks, planning outcomes, adoption behavior, auto-sync, and manual-value handling.

1. **Prepare the file.** Generate or update the repository's Blueprint at its intended path. Consult the current specification through the shared reference. Read [references/service-types.md](references/service-types.md), [references/runtimes.md](references/runtimes.md), and [references/environment-variables.md](references/environment-variables.md) as needed. Use [references/private-networking.md](references/private-networking.md) when wiring services; place related resources in the same region unless directed otherwise. Framework examples are in [references/deployment-details.md](references/deployment-details.md).
2. **Validate.** Run `bex blueprints validate <file>` when available, or use the current validation API/schema workflow in the shared reference. Fix errors. Local validation does not create a plan or deploy anything.
3. **Publish the intended revision.** Inspect the Git remote, branch, and diff. Commit and push only within the user's authorization, preserving unrelated work. Verify the file exists remotely at the selected branch and repository-relative path. A merge into `main` is not required. For an already approved Blueprint, check auto-sync before pushing: that push may itself apply changes.
4. **Create or plan.** For a new Blueprint, use `create_blueprint` or `bex ea blueprints create`; for an existing one, use `plan_blueprint` or `bex ea blueprints plan`. Creation already plans the first sync. Pass the intended branch explicitly when supplying a repo URL. Choose separate resources versus adoption and explain future auto-sync behavior before proceeding.
5. **Review the result.** Handle `planningOutcome` as described in the shared reference. For `planned`, summarize the proposed resources, updates, adoption, notices, and required manual variables; share the returned Dashboard URL. For `failed`, fix the cause and re-plan the existing Blueprint. For `no_changes`, do not approve.
6. **Apply when authorized.** After the user authorizes applying the reviewed plan, call `approve_blueprint` or `bex ea blueprints approve` with the Blueprint ID and required manual values. Reuse authorization that already covers this reviewed plan. Providing secrets or accepting costs alone is not deployment approval. Never chain approval blindly after creation or planning. Pass secret values only through the required input mechanism; in chat and final reports, name the variables without repeating their values.
7. **Verify the workloads.** Approval returns before the sync and deployments finish. Follow the verification workflow below; do not report a successful deployment from approval or Blueprint status alone.

For a settings change, follow the update semantics in [references/blueprints.md](references/blueprints.md). Changing branch/path can produce a plan; changing name/auto-sync alone does not. Settings can persist even if planning fails.

### Dashboard fallback

Use [references/blueprint-dashboard.md](references/blueprint-dashboard.md) when programmatic Blueprint tools are unavailable, the user prefers Dashboard approval, or Git authorization needs browser interaction. Provide the exact remaining action and resume verification after it is complete.

## Direct creation

1. Analyze the codebase and confirm the source and required resource configuration.
2. Use the available MCP creation tool or CLI `bex services create` after inspecting its schema/help. [references/direct-creation.md](references/direct-creation.md) contains MCP examples. Creation can immediately provision and deploy resources, so ensure it is within the user's authorized scope.
3. Supply required variables using [references/environment-variables.md](references/environment-variables.md), preferably during creation when supported. If repo access fails, guide the user to connect the Git provider, then retry after access is resolved.
4. Record the returned service/deploy IDs and verify the resulting workloads.

## Verify and report

Read [references/deployments.md](references/deployments.md) to monitor the specific resulting deploys through terminal states and verify application behavior. For Blueprints, use `get_blueprint` when available to identify managed resources, then inspect their individual deployments, logs, and health. CLI users can inspect resources with `bex services` and `bex deploys`.

Use [references/post-deploy-checks.md](references/post-deploy-checks.md) for the checklist and [references/troubleshooting-basics.md](references/troubleshooting-basics.md) for basic triage. Use `bex-debug` for deeper diagnosis.

Report the Blueprint/service IDs, useful Dashboard/application URLs, observed sync/deploy states, checks performed, and any unresolved failures or user action. Distinguish a prepared file, a pending plan, an approved sync, and a verified running application. Report which environment variables were supplied by name only; never echo their values.

<!-- shared:documentation-retrieval -->
## Current documentation retrieval

bex implements Render's documented platform behavior, so this skill links Render's documentation as the reference for how a feature behaves, and bex's own documentation (`https://bex.co/docs`) for its CLI and MCP server. Read Render's pages for behavior only: operate through bex (`bex` CLI, `https://api.bex.co/mcp`, `https://dashboard.bex.co`), never through the Render CLI, MCP server or dashboard those pages mention. Where bex's behavior differs, bex's documentation wins.

Whenever this skill directs you to consult current documentation:

1. Retrieve the linked Markdown document directly with an available URL-fetching tool or HTTP client, such as `curl`. Do not substitute web-search summaries for the document.
2. Confirm that retrieval succeeded and returned the expected document, then read its contents. Saving a file or printing its path is not sufficient.
3. If the request fails or your tool cannot read the Markdown response, open and read the linked HTML version instead.
4. If neither version can be retrieved, disclose that the current reference is unavailable and follow any topic-specific fallback in the skill. Use bundled guidance only for stable constraints, and do not guess at changeable platform details.

When a task requires multiple references, apply this workflow to each one and distinguish the documents you verified from those that remain unavailable.
<!-- /shared:documentation-retrieval -->
