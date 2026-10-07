import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const triggerDeploy = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (doc: { definitions?: Array<{ name?: { value?: string } }> }) =>
    doc.definitions?.[0]?.name?.value === "TriggerDeploy"
      ? [triggerDeploy, { loading: false }]
      : [vi.fn(), { loading: false }],
}));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: (...a: unknown[]) => toastError(...a) },
}));

const ask = vi.fn();
vi.mock("@/common/providers/protected-retry-context", () => ({
  useAskForProtectedConfirmation: () => ask,
}));

import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";
import { codedGraphQLError } from "@/test/mocks/apollo";

const REFUSAL = codedGraphQLError(
  "PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED",
  { confirm: "sudo repoint service web", name: "web" },
);

beforeEach(() => {
  triggerDeploy.mockReset();
  toastError.mockReset();
  ask.mockReset();
});

// w4/m176: "Deploy a specific commit" swaps the running code, so a protected
// member refuses it until the server-issued phrase is echoed back.
describe("useTriggerDeploy protected-environment retry", () => {
  it("retries a commit deploy with the server's phrase", async () => {
    triggerDeploy
      .mockRejectedValueOnce(REFUSAL)
      .mockResolvedValueOnce({ data: { triggerDeploy: { id: "dep-2" } } });
    ask.mockResolvedValue("sudo repoint service web");
    const { result } = renderHook(() => useTriggerDeploy());

    let id: string | null = null;
    await act(async () => {
      id = await result.current.trigger("srv-1", { commitId: "abc123" });
    });

    expect(id).toBe("dep-2");
    expect(ask).toHaveBeenCalledWith("sudo repoint service web", "web");
    expect(triggerDeploy).toHaveBeenNthCalledWith(2, {
      variables: {
        serviceId: "srv-1",
        commitId: "abc123",
        deployMode: undefined,
        clearCache: undefined,
        confirm: "sudo repoint service web",
      },
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("returns null without an error toast when the phrase is dismissed", async () => {
    triggerDeploy.mockRejectedValueOnce(REFUSAL);
    ask.mockResolvedValue(null);
    const { result } = renderHook(() => useTriggerDeploy());

    let id: string | null = "unset";
    await act(async () => {
      id = await result.current.trigger("srv-1", { commitId: "abc123" });
    });

    expect(id).toBeNull();
    expect(triggerDeploy).toHaveBeenCalledTimes(1);
    expect(toastError).not.toHaveBeenCalled();
  });
});
