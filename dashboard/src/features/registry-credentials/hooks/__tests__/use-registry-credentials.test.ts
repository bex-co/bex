import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

import { useRegistryCredentials } from "@/features/registry-credentials/hooks/use-registry-credentials";

beforeEach(() => {
  mockUseQuery.mockReset();
});

describe("useRegistryCredentials", () => {
  it("maps registryCredentials to views, dropping nulls and rows without an id", () => {
    mockUseQuery.mockReturnValue({
      data: {
        registryCredentials: [
          {
            __typename: "RegistryCredential",
            id: "rgc-1",
            name: "GHCR prod",
            host: "ghcr.io",
            username: "alice",
            expiresAt: null,
            status: "active",
            createdAt: "2026-07-01T00:00:00Z",
          },
          null,
          {
            __typename: "RegistryCredential",
            id: null,
            name: "orphan",
            host: "x",
            username: "y",
            expiresAt: null,
            status: "active",
            createdAt: null,
          },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useRegistryCredentials());
    expect(result.current.credentials).toEqual([
      {
        id: "rgc-1",
        name: "GHCR prod",
        host: "ghcr.io",
        username: "alice",
        expiresAt: null,
        status: "active",
        createdAt: "2026-07-01T00:00:00Z",
      },
    ]);
  });

  it("defaults a blank name to the host (server-side default mirrored client-side for safety)", () => {
    mockUseQuery.mockReturnValue({
      data: {
        registryCredentials: [
          {
            __typename: "RegistryCredential",
            id: "rgc-1",
            name: null,
            host: "docker.io",
            username: "bob",
            expiresAt: null,
            status: "active",
            createdAt: null,
          },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useRegistryCredentials());
    expect(result.current.credentials[0].name).toBe("docker.io");
  });

  it("never requests or surfaces a secret field — the view type has none", () => {
    mockUseQuery.mockReturnValue({
      data: {
        registryCredentials: [
          {
            __typename: "RegistryCredential",
            id: "rgc-1",
            name: "x",
            host: "ghcr.io",
            username: "alice",
            expiresAt: null,
            status: "active",
            createdAt: null,
          },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useRegistryCredentials());
    expect(result.current.credentials[0]).not.toHaveProperty("secret");
    expect(result.current.credentials[0]).not.toHaveProperty("authToken");
  });

  it("returns an empty list (not a crash) while loading with no data yet", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useRegistryCredentials());
    expect(result.current.credentials).toEqual([]);
  });
});
