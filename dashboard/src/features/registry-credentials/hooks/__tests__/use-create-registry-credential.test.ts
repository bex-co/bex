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

const CREATED = {
  data: { createRegistryCredential: { id: "rgc-new", host: "ghcr.io" } },
};

describe("useCreateRegistryCredential", () => {
  // w4/m670: the Add credential dialog sends Render's registry, and the
  // caller gets the new credential's id to select it.
  it("sends the registry and resolves the new credential's id", async () => {
    const mutate = vi.fn().mockResolvedValue(CREATED);
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateRegistryCredential());
    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create({
        registry: "GITHUB",
        username: "alice",
        authToken: "hunter2",
        name: "GHCR prod",
      });
    });

    expect(id).toBe("rgc-new");
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        ownerId: null,
        registry: "GITHUB",
        host: null,
        username: "alice",
        authToken: "hunter2",
        name: "GHCR prod",
        expiresAt: null,
      },
    });
    // The host bex-api filled in from the registry, not a typed one.
    expect(toastSuccess).toHaveBeenCalledWith("Added a credential for ghcr.io");
    expect(toastError).not.toHaveBeenCalled();
  });

  it("sends a registry's host beside it, and a host alone", async () => {
    const mutate = vi.fn().mockResolvedValue(CREATED);
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateRegistryCredential());
    await act(async () => {
      await result.current.create({
        registry: "GOOGLE_ARTIFACT",
        host: "us-docker.pkg.dev",
        username: "_json_key",
        authToken: "{}",
      });
      await result.current.create({
        host: "registry.example.com:5000",
        username: "bob",
        authToken: "s3cr3t",
      });
    });

    expect(mutate.mock.calls.map(([{ variables }]) => variables)).toEqual([
      expect.objectContaining({
        registry: "GOOGLE_ARTIFACT",
        host: "us-docker.pkg.dev",
        name: null,
        expiresAt: null,
      }),
      expect.objectContaining({
        registry: null,
        host: "registry.example.com:5000",
      }),
    ]);
  });

  // w4/149: bex-api stores the canonical host, so the toast names that one.
  it("names the host the server stored, not the pasted URL", async () => {
    const mutate = vi.fn().mockResolvedValue(CREATED);
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateRegistryCredential());
    await act(async () => {
      await result.current.create({
        host: "https://ghcr.io/",
        username: "alice",
        authToken: "hunter2",
      });
    });

    expect(toastSuccess).toHaveBeenCalledWith("Added a credential for ghcr.io");
  });

  it("surfaces a mutation failure as an error toast and resolves null", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateRegistryCredential());
    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create({
        registry: "GITHUB",
        username: "alice",
        authToken: "hunter2",
      });
    });

    expect(id).toBeNull();
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't add the registry credential",
    );
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("treats an answer without a credential id as a failure", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: { createRegistryCredential: { id: null, host: "ghcr.io" } },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateRegistryCredential());
    let id: string | null | undefined;
    await act(async () => {
      id = await result.current.create({
        registry: "GITHUB",
        username: "alice",
        authToken: "hunter2",
      });
    });

    expect(id).toBeNull();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalled();
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

    let pending!: Promise<string | null>;
    act(() => {
      pending = result.current.create({
        registry: "GITHUB",
        username: "alice",
        authToken: "hunter2",
      });
    });
    expect(result.current.busy).toBe(true);

    await act(async () => {
      resolveMutate(CREATED);
      await pending;
    });
    expect(result.current.busy).toBe(false);
  });
});
