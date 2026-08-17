import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
  },
}));

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useTriggerDeploy", () => {
  it("resolves the new deploy's id on success (w9/m1/t004 — callers navigate to it)", async () => {
    const triggerDeploy = vi.fn().mockResolvedValue({
      data: {
        triggerDeploy: { id: "dep-new-1", status: "update_in_progress" },
      },
    });
    mockUseMutation.mockReturnValue([triggerDeploy, { loading: false }]);

    const { result } = renderHook(() => useTriggerDeploy());
    let id: string | null = null;
    await act(async () => {
      id = await result.current.trigger("web");
    });

    expect(id).toBe("dep-new-1");
    expect(toastSuccess).toHaveBeenCalled();
    expect(toastError).not.toHaveBeenCalled();
  });

  it("resolves null and toasts an error on a rejected trigger — never a deploy id to navigate to", async () => {
    const triggerDeploy = vi
      .fn()
      .mockRejectedValue(new Error("service is suspended"));
    mockUseMutation.mockReturnValue([triggerDeploy, { loading: false }]);

    const { result } = renderHook(() => useTriggerDeploy());
    let id: string | null = "dep-should-be-overwritten";
    await act(async () => {
      id = await result.current.trigger("web");
    });

    expect(id).toBeNull();
    expect(toastError).toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("resolves null when the mutation succeeds but the server omits an id (contract violation)", async () => {
    const triggerDeploy = vi
      .fn()
      .mockResolvedValue({ data: { triggerDeploy: null } });
    mockUseMutation.mockReturnValue([triggerDeploy, { loading: false }]);

    const { result } = renderHook(() => useTriggerDeploy());
    let id: string | null = "dep-should-be-overwritten";
    await act(async () => {
      id = await result.current.trigger("web");
    });

    expect(id).toBeNull();
  });

  it("refetches only the queries that show the deploy — never every active query", () => {
    mockUseMutation.mockReturnValue([vi.fn(), { loading: false }]);

    renderHook(() => useTriggerDeploy());

    expect(mockUseMutation).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        refetchQueries: ["Server", "Deploys", "ServiceEvents"],
        awaitRefetchQueries: true,
      }),
    );
  });

  it("passes commitId/deployMode through to the mutation variables", async () => {
    const triggerDeploy = vi.fn().mockResolvedValue({
      data: { triggerDeploy: { id: "dep-new-2" } },
    });
    mockUseMutation.mockReturnValue([triggerDeploy, { loading: false }]);

    const { result } = renderHook(() => useTriggerDeploy());
    await act(async () => {
      await result.current.trigger("web", {
        commitId: "abc123",
        deployMode: "deploy_only",
      });
    });

    expect(triggerDeploy).toHaveBeenCalledWith({
      variables: {
        serviceId: "web",
        commitId: "abc123",
        deployMode: "deploy_only",
      },
    });
  });
});
