import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
const mockClientQuery = vi.fn();
const mockApolloClient = { query: mockClientQuery };
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
  useApolloClient: () => mockApolloClient,
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
  },
}));

import {
  useEnvVarKeys,
  useRevealEnvVar,
  useEnvVarMutations,
  classifyEnvVarError,
} from "@/features/services/hooks/use-env-vars";

beforeEach(() => {
  mockUseQuery.mockReset();
  mockUseMutation.mockReset();
  mockClientQuery.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useEnvVarKeys", () => {
  it("maps the paged env-var envelope to keys only, dropping nulls; id falls back to key", () => {
    mockUseQuery.mockReturnValue({
      data: {
        envVars: [
          {
            envVar: { __typename: "EnvVarListValue", id: "FOO", key: "FOO" },
            cursor: "FOO",
          },
          {
            envVar: { __typename: "EnvVarListValue", id: null, key: "BAR" },
            cursor: "BAR",
          },
          null,
          {
            envVar: { __typename: "EnvVarListValue", id: "x", key: null },
            cursor: "x",
          },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvVarKeys("web"));
    expect(result.current.keys).toEqual([
      { id: "FOO", key: "FOO" },
      { id: "BAR", key: "BAR" }, // id defaulted to key
    ]);
  });

  it("returns an empty list (not a crash) when the service is null", () => {
    mockUseQuery.mockReturnValue({
      data: { envVars: null },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useEnvVarKeys("web"));
    expect(result.current.keys).toEqual([]);
  });

  it("walks every cursor page and appends each key once", async () => {
    const first = Array.from({ length: 100 }, (_, i) => {
      const key = `KEY_${String(i).padStart(3, "0")}`;
      return { envVar: { id: key, key }, cursor: key };
    });
    mockUseQuery.mockReturnValue({
      data: { envVars: first },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    mockClientQuery.mockResolvedValue({
      data: {
        envVars: [
          { envVar: { id: "KEY_100", key: "KEY_100" }, cursor: "KEY_100" },
          { envVar: { id: "KEY_101", key: "KEY_101" }, cursor: "KEY_101" },
        ],
      },
    });

    const { result } = renderHook(() => useEnvVarKeys("web"));
    await waitFor(() => expect(result.current.keys).toHaveLength(102));
    expect(new Set(result.current.keys.map((key) => key.id)).size).toBe(102);
    expect(mockClientQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { serviceId: "web", cursor: "KEY_099", limit: 100 },
      }),
    );
  });
});

describe("useRevealEnvVar", () => {
  it("fetches one value on demand with the right variables and returns it", async () => {
    mockClientQuery.mockResolvedValue({
      data: { service: { envVar: { value: "s3cret" } } },
    });
    const { result } = renderHook(() => useRevealEnvVar("web"));
    const value = await result.current("FOO");
    expect(value).toBe("s3cret");
    expect(mockClientQuery).toHaveBeenCalledWith(
      expect.objectContaining({ variables: { id: "web", key: "FOO" } }),
    );
  });

  it("returns '' when the value is absent", async () => {
    mockClientQuery.mockResolvedValue({ data: { service: { envVar: null } } });
    const { result } = renderHook(() => useRevealEnvVar("web"));
    expect(await result.current("FOO")).toBe("");
  });
});

describe("useEnvVarMutations", () => {
  it("setVar writes, refetches, and toasts success (with the redeploy note)", async () => {
    const setEnvVar = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [setEnvVar]);
    const refetch = vi.fn().mockResolvedValue([]);

    const { result } = renderHook(() => useEnvVarMutations("web", refetch));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setVar("FOO", "bar");
    });

    expect(ok).toBe(true);
    expect(setEnvVar).toHaveBeenCalledWith({
      variables: {
        serviceId: "web",
        key: "FOO",
        value: "bar",
        generateValue: undefined,
      },
    });
    expect(refetch).toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalledWith(
      "Saved FOO",
      expect.objectContaining({
        description: "The service is redeploying to apply the change.",
      }),
    );
  });

  it("setVar surfaces a failure as an error toast and resolves false", async () => {
    const setEnvVar = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockImplementation(() => [setEnvVar]);
    const refetch = vi.fn().mockResolvedValue([]);

    const { result } = renderHook(() => useEnvVarMutations("web", refetch));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setVar("FOO", "bar");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith("Couldn't save FOO");
    expect(refetch).not.toHaveBeenCalled();
  });

  it("setVar sends generateValue without a conflicting literal", async () => {
    const setEnvVar = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [setEnvVar]);
    const refetch = vi.fn().mockResolvedValue([]);
    const { result } = renderHook(() => useEnvVarMutations("web", refetch));

    await act(async () => {
      await result.current.setVar("TOKEN", "ignored", true);
    });

    expect(setEnvVar).toHaveBeenCalledWith({
      variables: {
        serviceId: "web",
        key: "TOKEN",
        value: undefined,
        generateValue: true,
      },
    });
  });
});

describe("classifyEnvVarError", () => {
  it("classifies the store-unavailable, forbidden, and generic errors, and null", () => {
    expect(classifyEnvVarError(undefined)).toBeNull();
    expect(classifyEnvVarError(new Error("secret store not configured"))).toBe(
      "unavailable",
    );
    expect(classifyEnvVarError(new Error("forbidden"))).toBe("forbidden");
    expect(classifyEnvVarError(new Error("boom"))).toBe("generic");
  });
});
