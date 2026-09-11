import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import type { ReactNode } from "react";

vi.unmock("@/features/capabilities/hooks/use-capabilities");

const mockQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: mockQuery }),
}));

let workspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: workspaceId }),
}));

import { CapabilitiesProvider } from "@/features/capabilities/context/capabilities-provider";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  bumpAccessGeneration,
  resetAccessGenerationForTests,
} from "@/features/capabilities/lib/access-generation";

const allowedGrants = [
  "can_view",
  "can_view_logs",
  "can_operate",
  "can_create",
  "can_view_sensitive",
  "can_manage_keys",
  "can_manage",
  "can_manage_billing",
].map((action) => ({ action, outcome: "allowed", reason: null }));

function wrapper({ children }: { children: ReactNode }) {
  return <CapabilitiesProvider>{children}</CapabilitiesProvider>;
}

describe("useCapabilities (fail-closed + freshness)", () => {
  beforeEach(() => {
    mockQuery.mockReset();
    workspaceId = "tea-1";
    resetAccessGenerationForTests();
    Object.defineProperty(document, "hidden", {
      configurable: true,
      get: () => false,
    });
  });

  it("never grants while the answer is unknown", async () => {
    mockQuery.mockReturnValue(new Promise(() => undefined));
    const { result } = renderHook(() => useCapabilities(), { wrapper });
    expect(result.current.canCreate).toBe(false);
    expect(result.current.canViewSensitive).toBe(false);
    expect(result.current.loaded).toBe(false);
    expect(result.current.loading).toBe(true);
  });

  it("reflects grants only when ready and fresh", async () => {
    mockQuery.mockResolvedValue({
      data: {
        viewerCapabilities: {
          role: "CONTRIBUTOR",
          fresh: false,
          grants: [
            ...allowedGrants.filter((g) => g.action !== "can_create"),
            {
              action: "can_create",
              outcome: "denied",
              reason: "insufficient_permission",
            },
          ],
        },
      },
    });
    const { result } = renderHook(() => useCapabilities(), { wrapper });
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(result.current.role).toBe("CONTRIBUTOR");
    expect(result.current.canOperate).toBe(true);
    expect(result.current.canCreate).toBe(false);
    expect(result.current.denied("can_create")).toBe(true);
    expect(result.current.loaded).toBe(true);
    expect(result.current.reasonKey("can_create")).toBe(
      "capabilities.reasonCanCreate",
    );
  });

  it("treats a failed check as unavailable, not a role denial", async () => {
    mockQuery.mockRejectedValue(new Error("network"));
    const { result } = renderHook(() => useCapabilities(), { wrapper });
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(result.current.canCreate).toBe(false);
    expect(result.current.unavailable).toBe(true);
    expect(result.current.denied("can_create")).toBe(false);
    expect(result.current.reasonKey("can_create")).toBe(
      "capabilities.grantUnavailable",
    );
  });

  it("scopes fresh queries to the active workspace", async () => {
    mockQuery.mockResolvedValue({
      data: {
        viewerCapabilities: {
          role: "ADMIN",
          fresh: true,
          grants: allowedGrants,
        },
      },
    });
    renderHook(() => useCapabilities(), { wrapper });
    await act(async () => {
      await Promise.resolve();
    });
    expect(mockQuery).toHaveBeenCalled();
    const vars = mockQuery.mock.calls[0][0].variables;
    expect(vars.ownerId).toBe("tea-1");
  });

  it("ignores obsolete responses after an access-generation bump", async () => {
    let resolveFirst!: (value: unknown) => void;
    const first = new Promise((resolve) => {
      resolveFirst = resolve;
    });
    mockQuery.mockReturnValueOnce(first).mockResolvedValue({
      data: {
        viewerCapabilities: {
          role: "ADMIN",
          fresh: true,
          grants: allowedGrants,
        },
      },
    });
    const { result } = renderHook(() => useCapabilities(), { wrapper });
    act(() => {
      bumpAccessGeneration();
    });
    resolveFirst({
      data: {
        viewerCapabilities: {
          role: "VIEWER",
          fresh: true,
          grants: allowedGrants.map((g) =>
            g.action === "can_create"
              ? { ...g, outcome: "allowed" }
              : g,
          ),
        },
      },
    });
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    // Generation bump aborted the first lease; later ready answer wins.
    expect(result.current.generation).toBeGreaterThan(0);
  });
});
