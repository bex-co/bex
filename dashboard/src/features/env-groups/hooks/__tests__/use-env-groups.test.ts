import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
const mockClientQuery = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: mockClientQuery }),
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

// Scoped to the switcher's selection (w6/m24), never a workspace the hook
// resolves itself — same seam useApiKeys/useServices use.
let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import {
  classifyEnvGroupError,
  isEnvGroupNotFound,
  useEnvGroup,
  useEnvGroupMutations,
  useEnvGroupVarMutations,
  useEnvGroups,
  useRevealEnvGroupSecretFile,
  useRevealEnvGroupVar,
} from "@/features/env-groups/hooks/use-env-groups";

beforeEach(() => {
  mockUseQuery.mockReset();
  mockUseMutation.mockReset();
  mockClientQuery.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  currentWorkspaceId = "tea-1";
});

const wireGroup = {
  __typename: "EnvGroup" as const,
  id: "eg1",
  name: "shared",
  ownerId: "tea-1",
  createdAt: "2026-07-15T12:00:00Z",
  updatedAt: "2026-07-15T13:00:00Z",
  serviceLinks: ["web", null, "worker"],
  envVars: [{ key: "FOO" }, null, { key: null }],
  secretFiles: [{ name: "cert.pem" }, null],
};

describe("environment-group queries", () => {
  it("maps the nullable list shape to workspace group views", () => {
    mockUseQuery.mockReturnValue({
      data: {
        envGroups: [
          wireGroup,
          null,
          { __typename: "EnvGroup", id: null, name: "bad" },
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
        ownerId: "tea-1",
        createdAt: "2026-07-15T12:00:00Z",
        updatedAt: "2026-07-15T13:00:00Z",
        serviceLinks: ["web", "worker"],
        envVarKeys: ["FOO"],
        secretFileNames: ["cert.pem"],
      },
    ]);
  });

  it("returns an empty list instead of crashing on a null list", () => {
    mockUseQuery.mockReturnValue({
      data: { envGroups: null },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvGroups());

    expect(result.current.groups).toEqual([]);
  });

  it("sends the switcher's workspace as ownerId, skipped until it resolves", () => {
    currentWorkspaceId = null;
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvGroups());

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ variables: { ownerId: null }, skip: true }),
    );
    // Skip means loading stays true regardless of Apollo's own flag, so the
    // list page doesn't flash an empty state before the selection resolves.
    expect(result.current.loading).toBe(true);
  });

  it("maps one detail response and refetches the same shape", async () => {
    const refetch = vi.fn().mockResolvedValue({
      data: { envGroup: { ...wireGroup, name: "renamed" } },
    });
    mockUseQuery.mockReturnValue({
      data: { envGroup: wireGroup },
      loading: false,
      error: undefined,
      refetch,
    });

    const { result } = renderHook(() => useEnvGroup("eg1"));

    expect(result.current.group?.name).toBe("shared");
    await expect(result.current.refetch()).resolves.toMatchObject({
      id: "eg1",
      name: "renamed",
    });
  });

  it("treats an empty adapter detail object as not found", () => {
    mockUseQuery.mockReturnValue({
      data: { envGroup: { id: "", name: "" } },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvGroup("missing"));

    expect(result.current.group).toBeNull();
  });
});

describe("useEnvGroupMutations", () => {
  it("creates a group, refetches, and returns the new stable id", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: { createEnvGroup: { id: "eg-new" } },
    });
    mockUseMutation.mockImplementation(() => [mutate]);
    const refetch = vi.fn().mockResolvedValue([]);

    const { result } = renderHook(() => useEnvGroupMutations(refetch));
    let id: string | null = null;
    await act(async () => {
      id = await result.current.createGroup({
        name: "shared",
        envVars: [{ key: "TOKEN", value: "secret" }],
        secretFiles: [{ name: "ca.pem", content: "CERT" }],
        serviceIds: ["web"],
      });
    });

    expect(id).toBe("eg-new");
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        name: "shared",
        ownerId: "tea-1",
        envVars: [{ key: "TOKEN", value: "secret" }],
        secretFiles: [{ name: "ca.pem", content: "CERT" }],
        serviceIds: ["web"],
      },
    });
    expect(refetch).toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalledWith("Created shared");
  });

  it("refuses to create until the switcher's workspace resolves", async () => {
    currentWorkspaceId = null;
    const mutate = vi.fn();
    mockUseMutation.mockImplementation(() => [mutate]);

    const { result } = renderHook(() =>
      useEnvGroupMutations(vi.fn().mockResolvedValue([])),
    );
    let id: string | null = "unset";
    await act(async () => {
      id = await result.current.createGroup({
        name: "shared",
        envVars: [],
        secretFiles: [],
        serviceIds: [],
      });
    });

    expect(id).toBeNull();
    expect(mutate).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith("Couldn't create shared");
  });

  it("links a service and explains that linked services roll out", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [mutate]);

    const { result } = renderHook(() =>
      useEnvGroupMutations(vi.fn().mockResolvedValue([])),
    );
    await act(async () => {
      await result.current.linkGroup("eg1", "web");
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "eg1", serviceId: "web" },
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Service linked",
      expect.objectContaining({
        description: "Linked services are redeploying to apply the change.",
      }),
    );
  });

  it("surfaces a delete failure and does not refetch", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockImplementation(() => [mutate]);
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

  it("does not misreport a successful delete when a list refresh fails", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [mutate]);
    const refetch = vi.fn().mockRejectedValue(new Error("network down"));

    const { result } = renderHook(() => useEnvGroupMutations(refetch));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.deleteGroup("eg1");
    });

    expect(ok).toBe(true);
    expect(toastSuccess).toHaveBeenCalledWith("Environment group deleted");
    expect(toastError).not.toHaveBeenCalled();
  });
});

describe("sensitive value reveal", () => {
  it("reveals one variable value without putting it on the list query", async () => {
    mockClientQuery.mockResolvedValue({
      data: { envGroupVar: { value: "secret" } },
    });
    const { result } = renderHook(() => useRevealEnvGroupVar("eg1"));

    await expect(result.current("TOKEN")).resolves.toBe("secret");
    expect(mockClientQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "eg1", key: "TOKEN" },
        fetchPolicy: "network-only",
      }),
    );
  });

  it("reveals one secret-file body on demand", async () => {
    mockClientQuery.mockResolvedValue({
      data: { envGroupSecretFile: { content: "pem" } },
    });
    const { result } = renderHook(() => useRevealEnvGroupSecretFile("eg1"));

    await expect(result.current("cert.pem")).resolves.toBe("pem");
  });
});

describe("useEnvGroupVarMutations", () => {
  it("supports the existing replace-all contract and refetches", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [mutate]);
    const refetch = vi.fn().mockResolvedValue(undefined);
    const { result } = renderHook(() =>
      useEnvGroupVarMutations("eg1", refetch),
    );

    await act(async () => {
      await result.current.setVars([
        { key: "A", value: "one" },
        { key: "B", value: "two" },
      ]);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "eg1",
        envVars: [
          { key: "A", value: "one" },
          { key: "B", value: "two" },
        ],
      },
    });
    expect(refetch).toHaveBeenCalled();
  });
});

describe("classifyEnvGroupError", () => {
  it("distinguishes store, authorization, missing, and generic errors", () => {
    expect(classifyEnvGroupError(undefined)).toBeNull();
    expect(
      classifyEnvGroupError(new Error("secret store not configured")),
    ).toBe("unavailable");
    expect(classifyEnvGroupError(new Error("forbidden"))).toBe("forbidden");
    expect(isEnvGroupNotFound(new Error("env group not found"))).toBe(true);
    expect(isEnvGroupNotFound(new Error("boom"))).toBe(false);
    expect(classifyEnvGroupError(new Error("boom"))).toBe("generic");
  });
});
