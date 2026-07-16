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

// The workspace a create lands in is the switcher's selection (WorkspaceProvider),
// never one the hook resolves itself — w6/m14.
let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import {
  useCreateService,
  type CreateServiceResult,
} from "@/features/services/hooks/use-create-service";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  currentWorkspaceId = "tea-1";
});

describe("useCreateService", () => {
  it("sends the switcher's workspace as ownerId", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createService: { id: "srv-1" } } });
    mockUseMutation.mockReturnValue([mutate, { loading: false }]);

    const { result } = renderHook(() => useCreateService());
    let id: CreateServiceResult | null | undefined;
    await act(async () => {
      id = await result.current.create({
        name: "web",
        type: "web_service",
        environmentId: "env-production",
        image: "nginx",
        registryCredentialId: "rgc-private",
        runtime: "node",
        buildCommand: "npm ci",
        startCommand: "npm start",
        secretFiles: [{ name: "credentials.json", content: "secret" }],
      });
    });

    expect(id).toEqual({ id: "srv-1", deployId: undefined });
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: expect.objectContaining({
          name: "web",
          ownerId: "tea-1",
          type: "web_service",
          environmentId: "env-production",
          image: "nginx",
          registryCredentialId: "rgc-private",
          runtime: "node",
          buildCommand: "npm ci",
          startCommand: "npm start",
          secretFiles: [{ name: "credentials.json", content: "secret" }],
        }),
      }),
    );
    expect(toastSuccess).toHaveBeenCalledWith("Deploying web…");
  });

  it("threads Dockerfile Path through the createService mutation", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createService: { id: "srv-docker" } } });
    mockUseMutation.mockReturnValue([mutate, { loading: false }]);

    const { result } = renderHook(() => useCreateService());
    await act(async () => {
      await result.current.create({
        name: "docker-web",
        type: "web_service",
        repo: "https://github.com/x/mono",
        runtime: "docker",
        dockerfilePath: "docker/Dockerfile.prod",
        startCommand: "bin/server",
      });
    });

    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: expect.objectContaining({
          runtime: "docker",
          dockerfilePath: "docker/Dockerfile.prod",
          startCommand: "bin/server",
        }),
      }),
    );
  });

  it("follows a workspace switch — the next create carries the new ownerId", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { createService: { id: "srv-2" } } });
    mockUseMutation.mockReturnValue([mutate, { loading: false }]);

    const { result, rerender } = renderHook(() => useCreateService());
    currentWorkspaceId = "tea-2";
    rerender();

    await act(async () => {
      await result.current.create({ name: "web" });
    });

    expect(mutate.mock.calls[0][0].variables.ownerId).toBe("tea-2");
  });

  it("refuses to create until the workspace selection resolves", async () => {
    const mutate = vi.fn();
    mockUseMutation.mockReturnValue([mutate, { loading: false }]);
    currentWorkspaceId = null;

    const { result } = renderHook(() => useCreateService());
    let id: CreateServiceResult | null | undefined;
    await act(async () => {
      id = await result.current.create({ name: "web" });
    });

    // Sending a null ownerId would silently create in the caller's default
    // workspace — the very bug this wiring exists to prevent.
    expect(mutate).not.toHaveBeenCalled();
    expect(id).toBeNull();
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't create web. Please try again.",
    );
  });
});
