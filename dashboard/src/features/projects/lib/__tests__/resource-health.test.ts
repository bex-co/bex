import { describe, expect, it } from "vitest";
import { classifyResourceHealth } from "../resource-health";
import type { ResourceRow } from "../../types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";

function row(over: Partial<ResourceRow>): ResourceRow {
  return {
    kind: "database",
    id: "x",
    name: "x",
    createdAt: null,
    updatedAt: null,
    runtime: null,
    region: null,
    ...over,
  };
}

// A restarting datastore is work in progress, not a failure: the project
// overview must not raise "needs attention" over a config restart (w4/m137).
describe("classifyResourceHealth for a restarting datastore", () => {
  it("treats a Postgres config_restart as converging", () => {
    const database = {
      status: "config_restart",
      suspended: "not_suspended",
    } as DatabaseView;
    expect(classifyResourceHealth(row({ kind: "database", database }))).toBe(
      "converging",
    );
  });

  it("treats a Key Value config_restart as converging", () => {
    const keyValue = {
      status: "config_restart",
      suspended: false,
    } as KeyValueView;
    expect(classifyResourceHealth(row({ kind: "keyvalue", keyValue }))).toBe(
      "converging",
    );
  });
});
