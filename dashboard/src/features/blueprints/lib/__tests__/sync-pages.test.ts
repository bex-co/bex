import { describe, expect, it } from "vitest";
import {
  isBlueprintNavItemActive,
  isBlueprintSyncPage,
} from "@/features/blueprints/components/blueprint-nav";
import { blueprintSubpathLanding } from "@/features/blueprints/lib/blueprint-pages";
import {
  NO_CHANGES_PLAN_STATE,
  handedOverNoChangesPlan,
} from "@/features/blueprints/lib/manual-sync";
import {
  changingSyncActions,
  isApplyingSyncState,
  isSettledSyncState,
} from "@/features/blueprints/lib/sync-state";

// w3/m678: a sync's page is `/sync/<syncId>` (and `/sync` for a plan Manual
// sync handed over); Syncs is its open sidebar entry and its heading is the
// sync's own.
describe("a Blueprint sync's pages", () => {
  it.each([
    ["/blueprints/exs-1/sync/exe-9", true],
    ["/blueprints/exs-1/sync", true],
    ["/blueprints/exs-1/sync/", true],
    ["/blueprints/exs-1/syncs", false],
    ["/blueprints/exs-1/synced", false],
    ["/blueprints/exs-2/sync/exe-9", false],
  ])("%s is a sync's page: %s", (path, sync) => {
    expect(isBlueprintSyncPage(path, "exs-1")).toBe(sync);
  });

  it("highlights Syncs on a sync's page", () => {
    expect(
      isBlueprintNavItemActive(
        "/blueprints/exs-1/sync/exe-9",
        "exs-1",
        "/blueprints/$blueprintId/syncs",
      ),
    ).toBe(true);
  });

  it("lands only a path past a sync's page on Syncs", () => {
    expect(blueprintSubpathLanding("sync/exe-9/more")).toBe("/syncs");
    expect(blueprintSubpathLanding("no-such-page")).toBeNull();
  });

  it("reads a handed-over plan with no changes, and nothing without one", () => {
    expect(handedOverNoChangesPlan(NO_CHANGES_PLAN_STATE)).toBe(true);
    expect(handedOverNoChangesPlan({})).toBe(false);
    expect(handedOverNoChangesPlan(undefined)).toBe(false);
  });
});

describe("a sync's states and actions", () => {
  it("settles only on success or error", () => {
    expect(
      ["created", "pending", "running", "success", "error"].filter(
        isSettledSyncState,
      ),
    ).toEqual(["success", "error"]);
    expect(
      ["created", "pending", "running", "success"].filter(isApplyingSyncState),
    ).toEqual(["pending", "running"]);
  });

  it("lists the actions that change something, in plan order", () => {
    const action = (operation: string, name: string) => ({
      action: {
        operation,
        kind: "service",
        name,
        sourcePath: "",
        resourceId: null,
        changedFields: null,
        message: null,
      },
      state: "created",
    });
    expect(
      changingSyncActions([
        action("noop", "a"),
        action("update", "b"),
        action("create", "c"),
        action("detach", "d"),
        action("noop", "e"),
      ]).map((a) => a.action.name),
    ).toEqual(["b", "c", "d"]);
  });
});
