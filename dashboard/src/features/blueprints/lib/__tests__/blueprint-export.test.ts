import { describe, expect, it } from "vitest";
import type { HistoryState } from "@tanstack/history";
import {
  blueprintExportVariables,
  environmentTitleNames,
  exportableResources,
  orderedExportResources,
  readBlueprintExport,
  type BlueprintExport,
  type BlueprintExportResource,
} from "@/features/blueprints/lib/blueprint-export";
import type { ResourceRow } from "@/features/projects/types";

// w3/m681: Generate Blueprint hands `/blueprint/generate` its resources in
// the navigation's state, as Render's entry points do.

const web: BlueprintExportResource = {
  kind: "service",
  id: "srv-web",
  name: "web",
};
const pg: BlueprintExportResource = {
  kind: "database",
  id: "dpg-1",
  name: "pg",
};
const kv: BlueprintExportResource = {
  kind: "keyvalue",
  id: "red-1",
  name: "cache",
};
const group: BlueprintExportResource = {
  kind: "envgroup",
  id: "evg-1",
  name: "shared",
};

function row(kind: ResourceRow["kind"], id: string): ResourceRow {
  return {
    kind,
    id,
    name: `${kind}-${id}`,
    createdAt: null,
    updatedAt: null,
    runtime: null,
    region: null,
  };
}

describe("readBlueprintExport", () => {
  it("reads the export a navigation handed over", () => {
    const handed: BlueprintExport = {
      resources: [web, pg],
      source: "project environment menu",
      projectName: "storefront",
      environmentName: "Production",
    };
    expect(readBlueprintExport({ blueprintExport: handed })).toStrictEqual(
      handed,
    );
  });

  it("is null with nothing handed over: a pasted link or a server render", () => {
    expect(readBlueprintExport(undefined)).toBeNull();
    expect(readBlueprintExport({} as HistoryState)).toBeNull();
    expect(
      readBlueprintExport({
        blueprintExport: { resources: [], source: "action bar" },
      }),
    ).toBeNull();
  });

  it("drops what it cannot export, and is null when nothing is left", () => {
    const state = {
      blueprintExport: {
        resources: [
          web,
          { kind: "sandboxgroup", id: "sbg-1", name: "agents" },
          { kind: "service", id: 7, name: "bad id" },
          null,
        ],
        source: "action bar",
      },
    } as unknown as HistoryState;
    expect(readBlueprintExport(state)?.resources).toStrictEqual([web]);
    const none = {
      blueprintExport: {
        resources: [{ kind: "sandboxgroup", id: "x", name: "x" }],
      },
    } as unknown as HistoryState;
    expect(readBlueprintExport(none)).toBeNull();
  });

  it("reads an unknown source as the selection bar's, and drops non-string names", () => {
    const state = {
      blueprintExport: {
        resources: [web],
        source: "somewhere else",
        projectName: 3,
        environmentName: "Production",
      },
    } as unknown as HistoryState;
    expect(readBlueprintExport(state)).toStrictEqual({
      resources: [web],
      source: "action bar",
      environmentName: "Production",
    });
  });
});

describe("exportableResources", () => {
  it("keeps services, Postgres, Key Value and env groups, not sandbox groups", () => {
    const rows = [
      row("service", "a"),
      row("sandboxgroup", "b"),
      row("database", "c"),
      row("keyvalue", "d"),
      row("envgroup", "e"),
    ];
    expect(exportableResources(rows)).toStrictEqual([
      { kind: "service", id: "a", name: "service-a" },
      { kind: "database", id: "c", name: "database-c" },
      { kind: "keyvalue", id: "d", name: "keyvalue-d" },
      { kind: "envgroup", id: "e", name: "envgroup-e" },
    ]);
  });
});

describe("orderedExportResources and blueprintExportVariables", () => {
  const worker = { ...web, id: "srv-worker", name: "worker" };
  const mixed = [group, pg, web, kv, worker];

  it("orders services, then Postgres, Key Value and env groups, as Render names them", () => {
    expect(orderedExportResources(mixed).map((r) => r.name)).toStrictEqual([
      "web",
      "worker",
      "pg",
      "cache",
      "shared",
    ]);
  });

  it("splits the ids into generateBlueprint's lists", () => {
    expect(blueprintExportVariables(mixed)).toStrictEqual({
      serviceIds: ["srv-web", "srv-worker"],
      postgresIds: ["dpg-1"],
      keyValueIds: ["red-1"],
      envGroupIds: ["evg-1"],
    });
  });
});

describe("environmentTitleNames", () => {
  const names = { projectName: "storefront", environmentName: "Production" };

  it("names the project and environment only from an environment's menu", () => {
    expect(
      environmentTitleNames({
        resources: [web],
        source: "project environment menu",
        ...names,
      }),
    ).toStrictEqual({ project: "storefront", environment: "Production" });
    expect(
      environmentTitleNames({
        resources: [web],
        source: "action bar",
        ...names,
      }),
    ).toBeNull();
    expect(
      environmentTitleNames({
        resources: [web],
        source: "project environment menu",
        projectName: "storefront",
      }),
    ).toBeNull();
  });
});
