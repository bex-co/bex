import { describe, expect, it } from "vitest";
import {
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

describe("project resource filters", () => {
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
      filterProjectResources(rows, envGroups, {
        query: "api",
        kind: "services",
      }),
    ).toEqual({ rows: [rows[0]], envGroups: [] });
    expect(
      filterProjectResources(rows, envGroups, {
        query: "shared",
        kind: "envgroups",
      }),
    ).toEqual({ rows: [], envGroups });
  });
});
