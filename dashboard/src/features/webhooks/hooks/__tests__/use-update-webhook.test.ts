import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

import { useUpdateWebhook } from "@/features/webhooks/hooks/use-update-webhook";

async function save(mutate: ReturnType<typeof vi.fn>) {
  mockUseMutation.mockReturnValue([mutate]);
  const { result } = renderHook(() => useUpdateWebhook());
  let accepted: unknown;
  await act(async () => {
    accepted = await result.current.update("whk-1", { name: "  hook  " });
  });
  return accepted;
}

beforeEach(() => mockUseMutation.mockReset());

// w4/271: the settings form adopts the accepted (trimmed) Name/URL.
describe("useUpdateWebhook accepted result (w4/271)", () => {
  it("resolves the Name/URL the server accepted", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        updateWebhookEndpoint: {
          id: "whk-1",
          name: "hook",
          url: "https://example.com/hook",
        },
      },
    });
    expect(await save(mutate)).toEqual({
      name: "hook",
      url: "https://example.com/hook",
    });
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "whk-1", ownerId: "tea-1", name: "  hook  " },
    });
  });

  it("resolves null when the response carries no accepted record", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { updateWebhookEndpoint: null } });
    expect(await save(mutate)).toBeNull();
  });

  it("resolves null when the save fails", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("boom"));
    expect(await save(mutate)).toBeNull();
  });
});
