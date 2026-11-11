import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useRouterNavAvailable } from "../use-router-nav-available";

// test/setup.ts mocks the hook for every other test; this one is about it.
vi.unmock("@/common/components/dashboard-layout/use-router-nav-available");

const mocks = vi.hoisted(() => ({
  routerEnabled: true,
  useQuery: vi.fn(),
}));

vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mocks.useQuery(...args),
}));
vi.mock("@/config/use-growthbook", () => ({
  useRouterFeatureEnabled: () => mocks.routerEnabled,
}));
vi.mock("@/features/workspaces/context", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

// w21/042: the sidebar and the ⌘K palette offer Router under one gate, the
// router flag AND the companion service's own answer.
describe("useRouterNavAvailable", () => {
  beforeEach(() => {
    mocks.routerEnabled = true;
    mocks.useQuery.mockReset();
  });

  it("is true only when the flag is on and the service is available", () => {
    mocks.useQuery.mockReturnValue({ data: { routerAvailable: true } });
    expect(renderHook(() => useRouterNavAvailable()).result.current).toBe(true);
  });

  it("is false when the flag is on but the companion service is absent", () => {
    mocks.useQuery.mockReturnValue({ data: { routerAvailable: false } });
    expect(renderHook(() => useRouterNavAvailable()).result.current).toBe(
      false,
    );
  });

  it("is false until the probe answers", () => {
    mocks.useQuery.mockReturnValue({ data: undefined });
    expect(renderHook(() => useRouterNavAvailable()).result.current).toBe(
      false,
    );
  });

  it("does not probe a workspace without the flag, whatever is cached", () => {
    mocks.routerEnabled = false;
    mocks.useQuery.mockReturnValue({ data: { routerAvailable: true } });
    expect(renderHook(() => useRouterNavAvailable()).result.current).toBe(
      false,
    );
    expect(mocks.useQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ skip: true }),
    );
  });

  it("asks about the current workspace", () => {
    mocks.useQuery.mockReturnValue({ data: undefined });
    renderHook(() => useRouterNavAvailable());
    expect(mocks.useQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: { ownerId: "tea-1" },
        skip: false,
      }),
    );
  });
});
