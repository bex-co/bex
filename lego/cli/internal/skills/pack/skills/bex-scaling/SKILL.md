---
name: bex-scaling
description: >-
  Scales bex services—configures autoscaling targets, chooses instance
  compute plans, sets manual instance counts, and optimizes cost. Use when the
  user needs to handle more traffic, set up autoscaling, pick the right compute
  plan, reduce costs, or troubleshoot scaling behavior like slow scale-down or
  stuck instances.
license: MIT
compatibility: bex web services, private services, and background workers
metadata:
  author: bex
  version: "1.0.0"
  category: operations
---

# bex Scaling

This skill covers how to scale **Web Services**, **Private Services**, and **Background Workers** on bex: manual instance counts, autoscaling with a **Pro workspace or higher**, compute plan choices, and platform limits. Deeper guidance lives under `references/`.

When a service has or might need an attached disk, read `references/persistent-disks.md` before choosing a scaling architecture.

## When to Use

- Setting or changing **instance count** (Dashboard, CLI, API, or Blueprint)
- Configuring **autoscaling** (min/max, CPU and memory targets)
- Choosing **vertical** (plan) vs **horizontal** (more instances) scaling
- Understanding **constraints** (disks, static sites, cron/workflows, 100-instance cap)
- **Cost** implications of running more or larger instances
- **Blueprint** fields: `numInstances`, `scaling`, `plan`

## Manual Scaling

- Set **instance count** from **1 to 100** via the **Dashboard**, **CLI**, or **API**.
- **All instances share the same compute plan**; you cannot mix plans on one service.
- Changes apply **immediately**: bex **provisions** new instances and **deprovisions** excess capacity as needed.

## Autoscaling

Before recommending an autoscaling policy, establish the service type, workspace plan, whether the service is stateless or has a disk, the desired minimum and maximum instance counts, and whether CPU or memory is the limiting resource.

- Available only with a **Pro workspace or higher**.
- Configure **minimum** and **maximum** instances and targets for **CPU** and/or **memory** utilization (**1–90%** each).
- **At least one metric must be enabled** (CPU or memory). If **both** CPU and memory autoscaling toggles are **off**, autoscaling is **disabled**.
- If **both** manual instance settings and autoscaling are configured, **autoscaling wins**—manual count does not override the scaling policy in effect.

## Autoscaling Formula

bex computes a candidate instance count from utilization vs target:

`new_instances = ceil(current_instances * (current_utilization / target_utilization))`

- When **both** CPU and memory targets are set, the platform uses the **larger** of the two `new_instances` values (the more conservative scale-out).

## Scaling Constraints

| Constraint | Behavior |
|------------|----------|
| **Per service** | **Maximum 100** instances |
| **Persistent disk** | **Cannot** scale to multiple instances—disk-attached services stay **single-instance** |
| **Static sites** | **Not** scalable (served by CDN) |
| **Cron jobs & Workflows** | Scaling model **does not apply** (different execution model) |

## Scale-Down Behavior

- **Scale-up** is **immediate** when utilization supports it.
- **Scale-down** waits **a few minutes** after conditions allow reduction (**spike protection**). This reduces **flapping** from brief load spikes.

## Compute Plans

- In Blueprints, the compute plan is set with the **`plan`** field.
- See `references/compute-plans.md` for current terminology, canonical plan discovery, and guidance on scaling up versus out.

## Cost Patterns

- Scaling changes the amount of billable compute by changing the number and size of running instances.
- Confirm current compute charges at [bex pricing](https://bex.co/pricing).
- **Right-size** by monitoring **CPU and memory** utilization (see **bex-monitor**).

Sustained low CPU and memory across every instance indicates likely over-provisioning. Consult `references/compute-plans.md`, then test a smaller current compute plan, fewer manually scaled instances, or autoscaling that can scale down toward an appropriate utilization target (approximately 70% CPU is a common starting point for CPU-bound services). Change one dimension at a time when practical, and verify CPU, memory, latency, and errors under representative production traffic before keeping the change.

## Blueprint Configuration

**Manual instance count:**

```yaml
numInstances: 3
```

**Autoscaling:**

```yaml
scaling:
  minInstances: 1
  maxInstances: 10
  targetCPUPercent: 70
  targetMemoryPercent: 80
```

**Compute plan:**

```yaml
plan: 1c-2g
```

Do not rely on `numInstances` to cap autoscaling when a `scaling` block is present—**autoscaling takes precedence**. Preview behavior for scaling is detailed in `references/autoscaling-guide.md`.

## References

| Topic | File |
|--------|------|
| Compute plans and vertical vs horizontal scaling | `references/compute-plans.md` |
| Persistent disk constraints and storage alternatives | `references/persistent-disks.md` |
| Enabling autoscaling, targets, min/max, mistakes, previews | `references/autoscaling-guide.md` |

## Related Skills

- **bex-web-services** — Web Service settings, disks, deploy lifecycle
- **bex-background-workers** — Worker-specific configuration and scaling context
- **bex-blueprints** — Full Blueprint schema and field reference
- **bex-monitor** — Metrics, logs, and utilization for right-sizing

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
