import { describe, expect, it } from "vitest";
import { classifyResourceHealth } from "../resource-health";
import type { ResourceKind, ResourceRow } from "../../types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import type { ServiceView } from "@/features/services/types";

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

// w4/530: a service created seconds ago has no phase yet; its project said
// "1 resource needs attention" until the first reconcile landed.
describe("classifyResourceHealth for a just-created service", () => {
  it("treats a service with no phase yet as converging", () => {
    const service = {
      phase: "",
      suspended: false,
      type: "web_service",
      plan: "free",
    } as ServiceView;
    expect(classifyResourceHealth(row({ kind: "service", service }))).toBe(
      "converging",
    );
  });

  it("still flags an unrecognized phase", () => {
    const service = {
      phase: "WeirdNewPhase",
      suspended: false,
      type: "web_service",
      plan: "free",
    } as ServiceView;
    expect(classifyResourceHealth(row({ kind: "service", service }))).toBe(
      "attention",
    );
  });
});

// Every derived status key of every kind maps to one health, and a row whose
// view has not loaded yet is never "attention" (w4/530). This table is the
// classification contract the Overview's rolled-up line reads.
function serviceView(phase: string, suspended = false): ServiceView {
  return {
    phase,
    suspended,
    type: "web_service",
    plan: "free",
  } as ServiceView;
}
function databaseView(status: string): DatabaseView {
  return { status, suspended: "not_suspended" } as DatabaseView;
}
function keyValueView(status: string): KeyValueView {
  return { status, suspended: false } as KeyValueView;
}

describe("classifyResourceHealth for every kind and status key", () => {
  const serviceCases: Array<[string, ServiceView, string]> = [
    ["a running service", serviceView("running"), "healthy"],
    ["a suspended service", serviceView("running", true), "healthy"],
    ["a sleeping service", serviceView("hibernated"), "healthy"],
    ["a building service", serviceView("building"), "converging"],
    ["a deploying service", serviceView("deploying"), "converging"],
    ["a pending service", serviceView("pending"), "converging"],
    ["a failed service", serviceView("failed"), "attention"],
  ];
  it.each(serviceCases)("classifies %s", (_label, service, health) => {
    expect(classifyResourceHealth(row({ kind: "service", service }))).toBe(
      health,
    );
  });

  const databaseCases: Array<[string, DatabaseView, string]> = [
    ["an available database", databaseView("available"), "healthy"],
    ["a creating database", databaseView("creating"), "converging"],
    ["a restarting database", databaseView("config_restart"), "converging"],
    ["an upgrading database", databaseView("upgrading"), "converging"],
    ["an unavailable database", databaseView("unavailable"), "attention"],
  ];
  it.each(databaseCases)("classifies %s", (_label, database, health) => {
    expect(classifyResourceHealth(row({ kind: "database", database }))).toBe(
      health,
    );
  });

  const keyValueCases: Array<[string, KeyValueView, string]> = [
    ["an available Key Value", keyValueView("available"), "healthy"],
    ["a creating Key Value", keyValueView("creating"), "converging"],
    ["a restarting Key Value", keyValueView("config_restart"), "converging"],
    ["an unavailable Key Value", keyValueView("unavailable"), "attention"],
  ];
  it.each(keyValueCases)("classifies %s", (_label, keyValue, health) => {
    expect(classifyResourceHealth(row({ kind: "keyvalue", keyValue }))).toBe(
      health,
    );
  });

  it("treats a row whose view has not loaded yet as healthy", () => {
    const kinds: ResourceKind[] = [
      "service",
      "database",
      "keyvalue",
      "envgroup",
    ];
    for (const kind of kinds) {
      expect(classifyResourceHealth(row({ kind }))).toBe("healthy");
    }
  });
});
