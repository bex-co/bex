import {
  buildResourceGroups,
  filterResourceGroups,
  statusTone,
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
