import { describe, expect, it } from "vitest";
import {
  countProjectResources,
  filterProjectResources,
  parseProjectResourceKind,
  parseProjectResourceSearch,
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
    {
      kind: "all",
      query: "",
      ids: ["srv-api", "db-main", "red-cache", "eg-shared"],
    },
    { kind: "services", query: "  PUBLIC  ", ids: ["srv-api"] },
    { kind: "databases", query: "DB-MAIN", ids: ["db-main"] },
    { kind: "keyvalues", query: "cache", ids: ["red-cache"] },
    { kind: "envgroups", query: "eg-shared", ids: ["eg-shared"] },
    {
      kind: "all",
      query: "  A  ",
      ids: ["srv-api", "db-main", "red-cache", "eg-shared"],
    },
    { kind: "services", query: "database", ids: [] },
    { kind: "all", query: "missing", ids: [] },
  ] as const)(
    "filters $kind by '$query' in input order",
    ({ kind, query, ids }) => {
      expect(
        filterProjectResources(allRows, { kind, query }).map((row) => row.id),
      ).toEqual(ids);
    },
  );

  it("counts repeated rows of every kind and empty input", () => {
    expect(countProjectResources([...allRows, allRows[2]])).toEqual({
      all: 5,
      services: 1,
      databases: 1,
      keyvalues: 2,
      envgroups: 1,
    });
    expect(countProjectResources([])).toEqual({
      all: 0,
      services: 0,
      databases: 0,
      keyvalues: 0,
      envgroups: 0,
    });
  });

  it("parses only URL-supported kinds", () => {
    expect(parseProjectResourceKind("envgroups")).toBe("envgroups");
    expect(parseProjectResourceKind("workers")).toBe("all");
  });

  it("restores valid URL-owned environment, query, and kind state", () => {
    expect(
      parseProjectResourceSearch({
        env: "env-production",
        q: "api",
        kind: "services",
      }),
    ).toEqual({ env: "env-production", q: "api", kind: "services" });
    expect(
      parseProjectResourceSearch({ env: 42, q: "", kind: "workers" }),
    ).toEqual({});
  });

  it("composes search and kind without escaping the supplied environment members", () => {
    expect(
      filterProjectResources([...rows, ...envGroupRows], {
        query: "api",
        kind: "services",
      }),
    ).toEqual([rows[0]]);
    expect(
      filterProjectResources([...rows, ...envGroupRows], {
        query: "shared",
        kind: "envgroups",
      }),
    ).toEqual(envGroupRows);
  });

  it("counts every selected-environment category before search", () => {
    expect(countProjectResources([...rows, ...envGroupRows])).toEqual({
      all: 3,
      services: 1,
      databases: 1,
      keyvalues: 0,
      envgroups: 1,
    });
  });
});
