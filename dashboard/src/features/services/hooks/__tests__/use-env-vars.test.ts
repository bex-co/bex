import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
const mockClientQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
  useApolloClient: () => ({ query: mockClientQuery }),
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
  it("maps the nested envVarKeys to a keys-only list, dropping nulls; id falls back to key", () => {
    mockUseQuery.mockReturnValue({
      data: {
        service: {
          __typename: "Service",
          id: "web",
          envVarKeys: [
            { __typename: "EnvVar", id: "FOO", key: "FOO" },
            { __typename: "EnvVar", id: null, key: "BAR" },
            null,
            { __typename: "EnvVar", id: "x", key: null },
          ],
        },
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
      data: { service: null },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useEnvVarKeys("web"));
    expect(result.current.keys).toEqual([]);
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
      variables: { serviceId: "web", key: "FOO", value: "bar" },
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
