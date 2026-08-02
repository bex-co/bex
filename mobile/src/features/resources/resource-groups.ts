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
): ReturnType<typeof buildResourceGroups> {
  if (kind === "all") return grouped;
  return {
    groups: grouped.groups.map((group) => ({
      ...group,
      resources: group.resources.filter((resource) => resource.kind === kind),
    })),
    ungrouped: grouped.ungrouped.filter((resource) => resource.kind === kind),
  };
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
