import { describe, expect, it } from "vitest";
import {
  countProjectResources,
  countsWithUnknown,
  environmentFilterCounts,
  matchesResourceQuery,
  parseEnvironmentResourceFilter,
  parseLegacyProjectSearch,
} from "../resource-filter";
import type { ResourceRow } from "../../types";
import type { EnvGroupView } from "@/features/env-groups/types";

const rows = [
  { kind: "service", id: "srv-api", name: "Public API" },
  { kind: "database", id: "db-main", name: "Primary database" },
] as ResourceRow[];
const envGroups = [
  { id: "eg-shared", name: "Shared config" },
] as EnvGroupView[];
const envGroupRows = [
  {
    kind: "envgroup",
    id: envGroups[0].id,
    name: envGroups[0].name,
  },
] as ResourceRow[];
const allRows = [
  ...rows,
  { kind: "keyvalue", id: "red-cache", name: "Cache" } as ResourceRow,
  ...envGroupRows,
];

describe("project resource filters", () => {
  it.each([
    { query: "", ids: ["srv-api", "db-main", "red-cache", "eg-shared"] },
    { query: "  PUBLIC  ", ids: ["srv-api"] },
    { query: "DB-MAIN", ids: ["db-main"] },
    { query: "eg-shared", ids: ["eg-shared"] },
    {
      query: "  A  ",
      ids: ["srv-api", "db-main", "red-cache", "eg-shared"],
    },
    { query: "missing", ids: [] },
  ] as const)("matches '$query' by name or id", ({ query, ids }) => {
    expect(
      allRows
        .filter((row) => matchesResourceQuery(row, query))
        .map((r) => r.id),
    ).toEqual(ids);
  });

  it("counts repeated rows of every kind and empty input", () => {
    expect(countProjectResources([...allRows, allRows[2]])).toEqual({
      all: 5,
      services: 1,
      databases: 1,
      keyvalues: 2,
      envgroups: 1,
      sandboxgroups: 0,
    });
    expect(countProjectResources([])).toEqual({
      all: 0,
      services: 0,
      databases: 0,
      keyvalues: 0,
      envgroups: 0,
      sandboxgroups: 0,
    });
  });

  it("parses only Render's three switcher choices", () => {
    expect(parseEnvironmentResourceFilter("envgroups")).toBe("envgroups");
    expect(parseEnvironmentResourceFilter("services")).toBe("services");
    // bex's pre-w1/m682 kinds are no choice of Render's switcher.
    expect(parseEnvironmentResourceFilter("databases")).toBe("all");
    expect(parseEnvironmentResourceFilter("keyvalues")).toBe("all");
    expect(parseEnvironmentResourceFilter(42)).toBe("all");
    expect(parseEnvironmentResourceFilter(undefined)).toBe("all");
  });

  it("reads the pre-w1/m682 query keys so their links can redirect", () => {
    expect(
      parseLegacyProjectSearch({
        env: "env-davm6imde41s73canq0g",
        q: "api",
        kind: "services",
      }),
    ).toStrictEqual({
      env: "evm-davm6imde41s73canq0g",
      q: "api",
      kind: "services",
    });
    // w5/078: undefined, not omitted, or the router keeps the raw value.
    expect(
      parseLegacyProjectSearch({ env: 42, q: "", kind: ["x"] }),
    ).toStrictEqual({ env: undefined, q: undefined, kind: undefined });
  });

  // Render's Services category counts services, Postgres, Key Value and
  // sandbox groups (w1/m682).
  it("counts Render's Services category across every non-group kind", () => {
    const groupRow = {
      kind: "sandboxgroup",
      id: "sbg-agents",
      name: "agents",
    } as ResourceRow;
    expect(
      environmentFilterCounts(countProjectResources([...allRows, groupRow])),
    ).toEqual({ all: 5, services: 4, envgroups: 1 });
  });

  it("leaves a category unknown while any family it counts is unread (w4/m227)", () => {
    const counts = countProjectResources(allRows);
    expect(
      environmentFilterCounts(countsWithUnknown(counts, ["keyvalues"])),
    ).toEqual({ all: null, services: null, envgroups: 1 });
    expect(
      environmentFilterCounts(countsWithUnknown(counts, ["envgroups"])),
    ).toEqual({ all: null, services: 3, envgroups: null });
    expect(environmentFilterCounts(countsWithUnknown(counts, []))).toEqual({
      all: 4,
      services: 3,
      envgroups: 1,
    });
  });
});
