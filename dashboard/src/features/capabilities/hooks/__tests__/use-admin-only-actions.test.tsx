import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";
import type { ResourceActionsState } from "@/features/capabilities/hooks/use-resource-actions";

vi.unmock("@/features/capabilities/hooks/use-admin-only-actions");

const projection = vi.hoisted(() => ({
  state: null as unknown,
  asked: [] as unknown[][],
}));
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useAdminOnlyProjection: (...args: unknown[]) => {
    projection.asked.push(args);
    return projection.state;
  },
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

const { useAdminOnlyActions } =
  await import("@/features/capabilities/hooks/use-admin-only-actions");

function ready(
  id: string,
  rows: { action: string; outcome: string; reason: string | null }[],
): ResourceActionsState {
  return {
    status: "ready",
    snapshot: toResourceSnapshot(
      "tea-test",
      id,
      rows.map((row) => ({ ...row, precondition: null })),
    ),
    refresh: async () => undefined,
  };
}

beforeEach(() => {
  projection.asked = [];
});

// w4/m594: each control a protected environment keeps to Admins reads the
// resource's own projection, so the reason it shows is bex-api's decision.
describe("useAdminOnlyActions", () => {
  it("names the admin-only reason a protected Developer is refused with", () => {
    projection.state = ready("evg-1", [
      {
        action: "update_env_vars",
        outcome: "denied",
        reason: "protected_environment_admin_required",
      },
      { action: "view_env_vars", outcome: "allowed", reason: null },
    ]);
    const { result } = renderHook(() =>
      useAdminOnlyActions("envGroup", "evg-1"),
    );
    expect(projection.asked).toContainEqual(["envGroup", "evg-1"]);
    expect(result.current("update_env_vars")).toBe(
      "This action is limited to admins.",
    );
    expect(result.current("view_env_vars")).toBeUndefined();
  });

  it("names a Viewer's role reason", () => {
    projection.state = ready("dpg-1", [
      {
        action: "view_connection_info",
        outcome: "denied",
        reason: "insufficient_permission",
      },
    ]);
    const { result } = renderHook(() =>
      useAdminOnlyActions("database", "dpg-1"),
    );
    expect(result.current("view_connection_info")).toBe(
      "Your role can’t perform this action. Ask a workspace admin.",
    );
  });

  it("fails closed while checking, for a row it never got, and for another resource", () => {
    projection.state = { status: "checking", refresh: async () => undefined };
    const checking = renderHook(() => useAdminOnlyActions("keyValue", "red-1"));
    expect(checking.result.current("update_ip_allow_list")).toBe(
      "Checking whether you can perform this action…",
    );

    projection.state = ready("red-2", [
      { action: "update_ip_allow_list", outcome: "allowed", reason: null },
    ]);
    const other = renderHook(() => useAdminOnlyActions("keyValue", "red-1"));
    expect(other.result.current("update_ip_allow_list")).toBeDefined();
  });

  it("refuses nothing without a resource", () => {
    projection.state = { status: "checking", refresh: async () => undefined };
    const { result } = renderHook(() => useAdminOnlyActions("envGroup", null));
    expect(result.current("delete")).toBeUndefined();
  });
});
