import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

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

let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useCreateWebhook } from "@/features/webhooks/hooks/use-create-webhook";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  currentWorkspaceId = "tea-1";
});

describe("useCreateWebhook", () => {
  it("uses fetchPolicy no-cache — the signing secret must never enter Apollo's cache", () => {
    mockUseMutation.mockReturnValue([vi.fn()]);
    renderHook(() => useCreateWebhook());
    expect(mockUseMutation).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ fetchPolicy: "no-cache" }),
    );
  });

  it("resolves the created endpoint (with secret) and toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        createWebhookEndpoint: {
          id: "whk-1",
          name: "slack-bot",
          secret: "whsec_s3cret",
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateWebhook());
    let endpoint;
    await act(async () => {
      endpoint = await result.current.create(
        "slack-bot",
        "https://example.com/hook",
        ["deploy_started"],
      );
    });

    expect(endpoint).toEqual({
      id: "whk-1",
      name: "slack-bot",
      secret: "whsec_s3cret",
    });
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        name: "slack-bot",
        url: "https://example.com/hook",
        eventTypes: ["deploy_started"],
        ownerId: "tea-1",
      },
    });
    expect(toastSuccess).toHaveBeenCalled();
  });

  it("refuses to create until the workspace selection resolves", async () => {
    currentWorkspaceId = null;
    const mutate = vi.fn();
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateWebhook());
    let endpoint;
    await act(async () => {
      endpoint = await result.current.create("x", "https://x.example", [
        "deploy_started",
      ]);
    });

    // Sending a null ownerId would silently register in the caller's default
    // workspace — the bug this wiring exists to prevent (the API-key precedent).
    expect(mutate).not.toHaveBeenCalled();
    expect(endpoint).toBeNull();
    expect(toastError).toHaveBeenCalled();
  });

  it("treats a response with no secret as a failure (server contract violated)", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: { createWebhookEndpoint: { id: "whk-1", name: "x", secret: null } },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateWebhook());
    let endpoint;
    await act(async () => {
      endpoint = await result.current.create("x", "https://x.example", [
        "deploy_started",
      ]);
    });

    expect(endpoint).toBeNull();
    expect(toastError).toHaveBeenCalled();
  });
});
