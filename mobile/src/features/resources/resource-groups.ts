import type { ColorTheme } from "@/types/theme-props";

export type ResourceKind = "service" | "database" | "keyValue";

export type ResourceStatusItem = {
  id: string;
  name: string;
  kind: ResourceKind;
  type: string;
  status: string;
  latestDeployId: string | null;
  projectId: string | null;
  updatedAt: string | null;
};

export type ResourceProject = {
  id: string;
  name: string;
  serviceIds: readonly (string | null)[] | null;
  databaseIds: readonly (string | null)[] | null;
  keyValueIds: readonly (string | null)[] | null;
};

export type ResourceGroup = {
  id: string;
  name: string;
  resources: ResourceStatusItem[];
};

const ids = (values: readonly (string | null)[] | null): string[] =>
  (values ?? []).filter((value): value is string => Boolean(value));

export function buildResourceGroups(
  projects: readonly ResourceProject[],
  resources: readonly ResourceStatusItem[],
): { groups: ResourceGroup[]; ungrouped: ResourceStatusItem[] } {
  const byId = new Map(resources.map((resource) => [resource.id, resource]));
  const assigned = new Set<string>();
  const groups = projects.map((project) => {
    const orderedIds = [
      ...ids(project.serviceIds),
      ...ids(project.databaseIds),
      ...ids(project.keyValueIds),
    ];
    const projectResources = orderedIds
      .map((id) => byId.get(id))
      .filter((resource): resource is ResourceStatusItem => Boolean(resource));
    projectResources.forEach((resource) => assigned.add(resource.id));
    return { id: project.id, name: project.name, resources: projectResources };
  });

  // projectId is a useful fallback for servers that returned a resource before
  // its id-list projection converged. It never invents a project association.
  for (const resource of resources) {
    if (assigned.has(resource.id) || !resource.projectId) continue;
    const group = groups.find(
      (candidate) => candidate.id === resource.projectId,
    );
    if (group) {
      group.resources.push(resource);
      assigned.add(resource.id);
    }
  }

  return {
    groups,
    ungrouped: resources.filter((resource) => !assigned.has(resource.id)),
  };
}

export function filterResourceGroups(
  grouped: ReturnType<typeof buildResourceGroups>,
  kind: ResourceKind | "all",
  search = "",
): ReturnType<typeof buildResourceGroups> {
  const query = search.trim().toLowerCase();
  const matches = (resource: ResourceStatusItem) =>
    (kind === "all" || resource.kind === kind) &&
    (!query ||
      `${resource.name} ${resource.type} ${resource.id}`
        .toLowerCase()
        .includes(query));
  if (kind === "all" && !query) return grouped;
  return {
    groups: grouped.groups.map((group) => ({
      ...group,
      resources: group.resources.filter(matches),
    })),
    ungrouped: grouped.ungrouped.filter(matches),
  };
}

export function statusToneColor(
  status: string,
  theme: Pick<ColorTheme, "success" | "warning" | "error" | "mutedForeground">,
): string {
  return {
    success: theme.success,
    warning: theme.warning,
    error: theme.error,
    muted: theme.mutedForeground,
  }[statusTone(status)];
}

export function statusTone(
  status: string,
): "success" | "warning" | "error" | "muted" {
  const normalized = status.toLowerCase();
  if (["running", "available", "live", "succeeded"].includes(normalized)) {
    return "success";
  }
  if (["failed", "unavailable", "error", "canceled"].includes(normalized)) {
    return "error";
  }
  if (
    ["creating", "building", "deploying", "pending", "suspended"].includes(
      normalized,
    )
  ) {
    return "warning";
  }
  return "muted";
}

/** Count each resource once even when project projections overlap. Unknown
 * phases stay unknown; an empty workspace never implies healthy resources. */
export function summarizeResources(
  grouped: ReturnType<typeof buildResourceGroups>,
) {
  const resources = new Map(
    [
      ...grouped.groups.flatMap((group) => group.resources),
      ...grouped.ungrouped,
    ].map((resource) => [`${resource.kind}:${resource.id}`, resource]),
  );
  const summary = { healthy: 0, review: 0, unknown: 0 };
  for (const resource of resources.values()) {
    const tone = statusTone(resource.status);
    summary[
      tone === "success" ? "healthy" : tone === "muted" ? "unknown" : "review"
    ]++;
  }
  return summary;
}
