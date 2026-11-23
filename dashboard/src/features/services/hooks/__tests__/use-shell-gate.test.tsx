import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";
import type { ResourceActionsState } from "@/features/capabilities/hooks/use-resource-actions";

vi.unmock("@/features/services/hooks/use-shell-gate");

const projection = vi.hoisted(() => ({
  state: null as unknown,
  asked: [] as unknown[][],
}));
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useServerActions: (...args: unknown[]) => {
    projection.asked.push(args);
    return projection.state;
  },
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

const { useShellGate } =
  await import("@/features/services/hooks/use-shell-gate");

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

// w4/726: the Shell tab reads serverActions' shell row, bex-api's own rule
// for CreateShellSession and native SSH, before it mints a ticket.
describe("useShellGate", () => {
  it("lets a caller the projection allows open the shell", () => {
    projection.state = ready("srv-1", [
      { action: "shell", outcome: "allowed", reason: null },
    ]);
    const { result } = renderHook(() => useShellGate("srv-1"));
    expect(projection.asked).toContainEqual(["srv-1"]);
    expect(result.current.gate.kind).toBe("ready");
    expect(result.current.reason).toBeUndefined();
  });

  it("names the admin-only reason a protected Developer is refused with", () => {
    projection.state = ready("srv-1", [
      {
        action: "shell",
        outcome: "denied",
        reason: "protected_environment_admin_required",
      },
    ]);
    const { result } = renderHook(() => useShellGate("srv-1"));
    expect(result.current.gate.kind).toBe("denied");
    expect(result.current.reason).toBe("This action is limited to admins.");
  });

  it("names a Viewer's role reason", () => {
    projection.state = ready("srv-1", [
      {
        action: "shell",
        outcome: "denied",
        reason: "insufficient_permission",
      },
    ]);
    const { result } = renderHook(() => useShellGate("srv-1"));
    expect(result.current.reason).toBe(
      "Your role can’t perform this action. Ask a workspace admin.",
    );
  });

  it("is checking while the projection loads", () => {
    projection.state = { status: "checking", refresh: async () => undefined };
    const { result } = renderHook(() => useShellGate("srv-1"));
    expect(result.current.gate.kind).toBe("checking");
  });

  // Fail closed: an answer about another service, or one with no shell row,
  // never opens a terminal.
  it("never reads another service's answer, or a missing row, as allowed", () => {
    projection.state = ready("srv-other", [
      { action: "shell", outcome: "allowed", reason: null },
    ]);
    expect(
      renderHook(() => useShellGate("srv-1")).result.current.gate.kind,
    ).toBe("unavailable");
    projection.state = ready("srv-1", [
      { action: "restart", outcome: "allowed", reason: null },
    ]);
    expect(
      renderHook(() => useShellGate("srv-1")).result.current.gate.kind,
    ).toBe("unavailable");
  });
});
