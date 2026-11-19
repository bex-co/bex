import { describe, expect, it } from "vitest";
import {
  ALL_SELECTION,
  CLEARED_SELECTION,
  effectiveLevels,
  highestGrantableLevel,
  readOnlySelection,
  sameSelection,
  selectionFromPermissions,
  selectionToInput,
  summarizePermissions,
  withArea,
  withLevel,
  type PermissionsSummary,
} from "@/features/api-keys/lib/permissions";
import type { ApiKeyPermissionsView } from "@/features/api-keys/types";
import {
  ALL_LEVELS,
  CATALOG,
  READ_ONLY_LEVELS,
} from "@/features/api-keys/__tests__/catalog-fixtures";

const area = (id: string) => CATALOG.find((a) => a.id === id)!;
const services = area("services");

describe("the permission picker's presets (w9/m496, Render's All / Read only / Clear all)", () => {
  it("All shows each permission at its broadest grantable level and sends {all: true}", () => {
    expect(effectiveLevels(ALL_SELECTION, CATALOG)).toEqual(ALL_LEVELS);
    expect(selectionToInput(ALL_SELECTION, CATALOG)).toEqual({ all: true });
  });

  it("Read only grants Read wherever it is offered and grantable, and nothing else", () => {
    const selection = readOnlySelection(CATALOG);
    expect(selection).toEqual({ all: false, levels: READ_ONLY_LEVELS });
    // Audit logs offers Read, but only an Admin may grant it.
    expect(selection.levels).not.toHaveProperty("audit_logs");
    expect(selectionToInput(selection, CATALOG)).toEqual({
      grants: [
        { permission: "services", level: "read" },
        { permission: "services_env", level: "read" },
        { permission: "git", level: "read" },
      ],
    });
  });

  it("Clear all sends an empty grant list", () => {
    expect(selectionToInput(CLEARED_SELECTION, CATALOG)).toEqual({
      grants: [],
    });
  });
});

describe("editing a selection", () => {
  it("an edit after All becomes the explicit grants All showed, with that row changed", () => {
    const next = withLevel(ALL_SELECTION, CATALOG, "services", "read");
    expect(next.all).toBe(false);
    expect(selectionToInput(next, CATALOG)).toEqual({
      grants: [
        { permission: "services", level: "read" },
        { permission: "services_env", level: "read_write" },
        { permission: "sandbox_execution", level: "execute" },
        { permission: "git", level: "read" },
      ],
    });
  });

  it("None removes the permission", () => {
    const next = withLevel(
      readOnlySelection(CATALOG),
      CATALOG,
      "services",
      null,
    );
    expect(next.levels).not.toHaveProperty("services");
  });

  it("an area's toggle grants its permissions at their broadest level, or clears them", () => {
    const on = withArea(CLEARED_SELECTION, CATALOG, services, true);
    expect(on.levels).toEqual({
      services: "read_write",
      services_env: "read_write",
    });
    const off = withArea(on, CATALOG, services, false);
    expect(off.levels).toEqual({});
  });

  it("grants are sent in the catalog's order whatever order they were chosen in", () => {
    let selection = withLevel(CLEARED_SELECTION, CATALOG, "git", "read");
    selection = withLevel(selection, CATALOG, "services", "read");
    expect(selectionToInput(selection, CATALOG)).toEqual({
      grants: [
        { permission: "services", level: "read" },
        { permission: "git", level: "read" },
      ],
    });
  });

  it("the broadest grantable level is the last grantable one in the catalog's order", () => {
    expect(highestGrantableLevel(services.permissions[1])).toBe("read_write");
    // Git's Read & write needs an Admin: a Developer's broadest is Read.
    expect(highestGrantableLevel(area("git").permissions[0])).toBe("read");
    // Audit logs: nothing a Developer may grant.
    expect(highestGrantableLevel(area("audit_logs").permissions[0])).toBeNull();
  });
});

describe("a key's grant as the editor's starting value", () => {
  it("round-trips All and explicit grants", () => {
    expect(selectionFromPermissions({ all: true, grants: [] })).toEqual(
      ALL_SELECTION,
    );
    expect(
      selectionFromPermissions({
        all: false,
        grants: [{ permission: "services", level: "read" }],
      }),
    ).toEqual({ all: false, levels: { services: "read" } });
  });

  it("compares selections by what they grant", () => {
    expect(sameSelection(ALL_SELECTION, { all: true, levels: {} })).toBe(true);
    expect(sameSelection(ALL_SELECTION, CLEARED_SELECTION)).toBe(false);
    expect(
      sameSelection(
        { all: false, levels: { services: "read", git: "read" } },
        { all: false, levels: { git: "read", services: "read" } },
      ),
    ).toBe(true);
    expect(
      sameSelection(
        { all: false, levels: { services: "read" } },
        { all: false, levels: { services: "read_write" } },
      ),
    ).toBe(false);
  });
});

describe("the row summary", () => {
  it.each<[ApiKeyPermissionsView, PermissionsSummary]>([
    [{ all: true, grants: [] }, { kind: "all" }],
    [{ all: false, grants: [] }, { kind: "none" }],
    [
      {
        all: false,
        grants: [
          { permission: "services", level: "read" },
          { permission: "git", level: "read" },
        ],
      },
      { kind: "readOnly", count: 2 },
    ],
    [
      {
        all: false,
        grants: [
          { permission: "services", level: "read_write" },
          { permission: "git", level: "read" },
          { permission: "sandbox_execution", level: "execute" },
        ],
      },
      { kind: "custom", count: 3 },
    ],
  ])("%j is %j", (permissions, summary) => {
    expect(summarizePermissions(permissions)).toEqual(summary);
  });
});
