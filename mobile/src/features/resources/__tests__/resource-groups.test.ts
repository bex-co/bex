import {
  buildResourceGroups,
  filterResourceGroups,
  statusTone,
  summarizeResources,
  type ResourceStatusItem,
} from "../resource-groups";

const resources: ResourceStatusItem[] = [
  {
    id: "srv-one",
    name: "web",
    kind: "service",
    type: "Web service",
    status: "running",
    latestDeployId: "dep-one",
    projectId: "prj-one",
    updatedAt: null,
  },
  {
    id: "db-one",
    name: "postgres",
    kind: "database",
    type: "PostgreSQL 17",
    status: "available",
    latestDeployId: null,
    projectId: "prj-one",
    updatedAt: null,
  },
  {
    id: "kv-one",
    name: "cache",
    kind: "keyValue",
    type: "Valkey 8",
    status: "creating",
    latestDeployId: null,
    projectId: null,
    updatedAt: null,
  },
];

describe("resource grouping", () => {
  it("groups mixed resources and leaves honest ungrouped rows", () => {
    const result = buildResourceGroups(
      [
        {
          id: "prj-one",
          name: "Production",
          serviceIds: ["srv-one"],
          databaseIds: ["db-one"],
          keyValueIds: [],
        },
      ],
      resources,
    );
    expect(result.groups[0].resources.map((resource) => resource.id)).toEqual([
      "srv-one",
      "db-one",
    ]);
    expect(result.ungrouped.map((resource) => resource.id)).toEqual(["kv-one"]);
  });

  it("filters every group without changing its project identity", () => {
    const grouped = buildResourceGroups([], resources);
    expect(
      filterResourceGroups(grouped, "keyValue").ungrouped.map(
        (resource) => resource.id,
      ),
    ).toEqual(["kv-one"]);
  });

  it("uses stable health tones and preserves unknown states", () => {
    expect(statusTone("available")).toBe("success");
    expect(statusTone("failed")).toBe("error");
    expect(statusTone("creating")).toBe("warning");
    expect(statusTone("migrating_custom_state")).toBe("muted");
  });
});

describe("resource discovery and health summary", () => {
  it("combines a trimmed case-insensitive search with the resource type", () => {
    const grouped = buildResourceGroups([], resources);
    expect(
      filterResourceGroups(grouped, "all", "  POSTGRES  ").ungrouped.map(
        (r) => r.id,
      ),
    ).toEqual(["db-one"]);
    expect(
      filterResourceGroups(grouped, "service", "postgres").ungrouped,
    ).toEqual([]);
    expect(
      filterResourceGroups(grouped, "all", "kv-one").ungrouped.map(
        (r) => r.name,
      ),
    ).toEqual(["cache"]);
    expect(filterResourceGroups(grouped, "all", "  ").ungrouped).toEqual(
      resources,
    );
  });

  it("searches within projects without losing their labels or mutating the source", () => {
    const grouped = buildResourceGroups(
      [
        {
          id: "prj-one",
          name: "Production",
          serviceIds: ["srv-one"],
          databaseIds: ["db-one"],
          keyValueIds: [],
        },
      ],
      resources,
    );
    const filtered = filterResourceGroups(grouped, "database", "17");
    expect(filtered.groups[0].name).toBe("Production");
    expect(filtered.groups[0].resources.map((r) => r.id)).toEqual(["db-one"]);
    expect(grouped.groups[0].resources.length).toBe(2);
  });

  it("counts overlapping projections once and never treats an unknown phase as healthy", () => {
    const grouped = buildResourceGroups(
      [],
      [
        ...resources,
        { ...resources[0], id: "srv-unknown", status: "new_server_phase" },
        { ...resources[0], id: "srv-failed", status: "failed" },
      ],
    );
    grouped.groups.push({
      id: "prj-duplicate",
      name: "Duplicate projection",
      resources: [resources[0]],
    });
    expect(summarizeResources(grouped)).toEqual({
      healthy: 2,
      review: 2,
      unknown: 1,
    });
    expect(summarizeResources(buildResourceGroups([], []))).toEqual({
      healthy: 0,
      review: 0,
      unknown: 0,
    });
  });
});
