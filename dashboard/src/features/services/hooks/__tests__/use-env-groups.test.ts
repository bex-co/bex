import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
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

import {
  useEnvGroups,
  useEnvGroupMutations,
  classifyEnvGroupError,
} from "@/features/services/hooks/use-env-groups";

beforeEach(() => {
  mockUseQuery.mockReset();
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useEnvGroups", () => {
  it("maps the deeply-nullable envGroups wire shape to a flat view, dropping nulls", () => {
    mockUseQuery.mockReturnValue({
      data: {
        envGroups: [
          {
            __typename: "EnvGroup",
            id: "eg1",
            name: "shared",
            serviceLinks: ["web", null, "worker"],
            envVars: [{ key: "FOO" }, null, { key: null }],
            secretFiles: [{ name: "cert.pem" }, null],
          },
          null,
          { __typename: "EnvGroup", id: null, name: "bad" }, // dropped: no id
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvGroups());
    expect(result.current.groups).toEqual([
      {
        id: "eg1",
        name: "shared",
        serviceLinks: ["web", "worker"],
        envVarKeys: ["FOO"],
        secretFileNames: ["cert.pem"],
      },
    ]);
  });

  it("returns an empty list (not a crash) when envGroups is null", () => {
    mockUseQuery.mockReturnValue({
      data: { envGroups: null },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useEnvGroups());
    expect(result.current.groups).toEqual([]);
  });
});

describe("useEnvGroupMutations", () => {
  it("createGroup writes, refetches, and toasts success", async () => {
    const fn = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [fn]);
    const refetch = vi.fn().mockResolvedValue([]);

    const { result } = renderHook(() => useEnvGroupMutations(refetch));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.createGroup("shared");
    });

    expect(ok).toBe(true);
    expect(fn).toHaveBeenCalledWith({ variables: { name: "shared" } });
    expect(refetch).toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalledWith("Created shared");
  });

  it("linkGroup writes with id+serviceId, refetches, and toasts with the redeploy note", async () => {
    const fn = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [fn]);
    const refetch = vi.fn().mockResolvedValue([]);

    const { result } = renderHook(() => useEnvGroupMutations(refetch));
    await act(async () => {
      await result.current.linkGroup("eg1", "web");
    });

    expect(fn).toHaveBeenCalledWith({ variables: { id: "eg1", serviceId: "web" } });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Group linked",
      expect.objectContaining({
        description: "The service is redeploying to apply the change.",
      }),
    );
  });

  it("deleteGroup surfaces a failure as an error toast and resolves false", async () => {
    const fn = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockImplementation(() => [fn]);
    const refetch = vi.fn().mockResolvedValue([]);

    const { result } = renderHook(() => useEnvGroupMutations(refetch));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.deleteGroup("eg1");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith("Couldn't delete the group");
    expect(refetch).not.toHaveBeenCalled();
  });
});

describe("classifyEnvGroupError", () => {
  it("classifies the store-unavailable, forbidden, and generic errors, and null", () => {
    expect(classifyEnvGroupError(undefined)).toBeNull();
    expect(classifyEnvGroupError(new Error("secret store not configured"))).toBe(
      "unavailable",
    );
    expect(classifyEnvGroupError(new Error("forbidden"))).toBe("forbidden");
    expect(classifyEnvGroupError(new Error("boom"))).toBe("generic");
  });
});
