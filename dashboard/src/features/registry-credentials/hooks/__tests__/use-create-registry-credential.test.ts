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

import { useCreateRegistryCredential } from "@/features/registry-credentials/hooks/use-create-registry-credential";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useCreateRegistryCredential", () => {
  it("fires createRegistryCredential with the given fields and toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateRegistryCredential());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.create({
        host: "ghcr.io",
        username: "alice",
        authToken: "hunter2",
        name: "GHCR prod",
      });
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        host: "ghcr.io",
        username: "alice",
        authToken: "hunter2",
        name: "GHCR prod",
        expiresAt: null,
      },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Added a credential for ghcr.io");
    expect(toastError).not.toHaveBeenCalled();
  });

  it("normalizes an empty name/expiresAt to null (server default)", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateRegistryCredential());
    await act(async () => {
      await result.current.create({
        host: "docker.io",
        username: "bob",
        authToken: "s3cr3t",
      });
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        host: "docker.io",
        username: "bob",
        authToken: "s3cr3t",
        name: null,
        expiresAt: null,
      },
    });
  });

  it("surfaces a mutation failure as an error toast and resolves false", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateRegistryCredential());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.create({
        host: "ghcr.io",
        username: "alice",
        authToken: "hunter2",
      });
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't add the credential for ghcr.io",
    );
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("tracks busy only for the duration of the in-flight mutation", async () => {
    let resolveMutate: (v: unknown) => void = () => {};
    const mutate = vi.fn(
      () =>
        new Promise((resolve) => {
          resolveMutate = resolve;
        }),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateRegistryCredential());
    expect(result.current.busy).toBe(false);

    let pending!: Promise<boolean>;
    act(() => {
      pending = result.current.create({
        host: "ghcr.io",
        username: "alice",
        authToken: "hunter2",
      });
    });
    expect(result.current.busy).toBe(true);

    await act(async () => {
      resolveMutate({});
      await pending;
    });
    expect(result.current.busy).toBe(false);
  });
});
