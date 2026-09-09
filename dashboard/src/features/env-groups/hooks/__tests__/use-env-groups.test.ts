import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
const mockClientQuery = vi.fn();
const mockCacheModify = vi.fn();
const mockCacheIdentify = vi.fn(() => "EnvGroup:eg1");
const mockCacheEvict = vi.fn();
const mockCacheGC = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({
    query: mockClientQuery,
    cache: {
      modify: mockCacheModify,
      identify: mockCacheIdentify,
      evict: mockCacheEvict,
      gc: mockCacheGC,
    },
  }),
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
  useEnvGroupEnvironmentPatch,
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
  mockCacheModify.mockReset();
  mockCacheIdentify.mockReset().mockReturnValue("EnvGroup:eg1");
  mockCacheEvict.mockReset();
  mockCacheGC.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  currentWorkspaceId = "tea-1";
});

const wireGroup = {
  __typename: "EnvGroup" as const,
  id: "eg1",
  name: "shared",
  ownerId: "tea-1",
  environmentId: "env-prod",
  createdAt: "2026-07-15T12:00:00Z",
  updatedAt: "2026-07-15T13:00:00Z",
  revision: "egr1_test",
  availability: null,
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
        environmentId: "env-prod",
        createdAt: "2026-07-15T12:00:00Z",
        updatedAt: "2026-07-15T13:00:00Z",
        revision: "egr1_test",
        availability: null,
        serviceLinks: ["web", "worker"],
        envVarKeys: ["FOO"],
        secretFileNames: ["cert.pem"],
      },
    ]);
  });

  it("treats an empty-string environmentId as null (w6/m48 defensive normalization)", () => {
    mockUseQuery.mockReturnValue({
      data: { envGroups: [{ ...wireGroup, environmentId: "" }] },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvGroups());

    expect(result.current.groups[0]?.environmentId).toBeNull();
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
    expect(result.current.loading).toBe(false);
  });

  it("keeps a cached empty list visible during a network refresh", () => {
    mockUseQuery.mockReturnValue({
      data: { envGroups: [] },
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvGroups());

    expect(result.current.groups).toEqual([]);
    expect(result.current.loading).toBe(false);
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

  it("evicts cached Environment memberships after moving a group", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [mutate]);
    const refetch = vi.fn().mockResolvedValue(undefined);
    const { result } = renderHook(() => useEnvGroupMutations(refetch));

    await act(async () => {
      await result.current.moveGroup("eg1", "env-prod");
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "eg1", environmentId: "env-prod" },
    });
    expect(mockCacheEvict).toHaveBeenCalledWith({
      id: "ROOT_QUERY",
      fieldName: "environments",
    });
    expect(refetch).toHaveBeenCalled();
  });

  it("invalidates the destination list after a server-side clone", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: { cloneEnvGroup: { id: "eg-clone" } },
    });
    mockUseMutation.mockImplementation(() => [mutate]);
    const { result } = renderHook(() =>
      useEnvGroupMutations(vi.fn().mockResolvedValue(undefined)),
    );
    let cloneID: string | null = null;

    await act(async () => {
      cloneID = await result.current.cloneGroup("eg1", "copy", "tea-2", null);
    });

    expect(cloneID).toBe("eg-clone");
    expect(mockCacheEvict).toHaveBeenCalledWith({
      id: "ROOT_QUERY",
      fieldName: "envGroups",
      args: { ownerId: "tea-2" },
    });
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
    expect(mockCacheModify).toHaveBeenCalled();
    expect(mockCacheEvict).toHaveBeenCalledWith({ id: "EnvGroup:eg1" });
    expect(mockCacheGC).toHaveBeenCalled();
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
        fetchPolicy: "no-cache",
      }),
    );
  });

  it("reveals one secret-file body on demand", async () => {
    mockClientQuery.mockResolvedValue({
      data: { envGroupSecretFile: { content: "pem" } },
    });
    const { result } = renderHook(() => useRevealEnvGroupSecretFile("eg1"));

    await expect(result.current("cert.pem")).resolves.toBe("pem");
    expect(mockClientQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "eg1", name: "cert.pem" },
        fetchPolicy: "no-cache",
      }),
    );
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

  it("preserves generated-value intent instead of sending an empty literal", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [mutate]);
    const { result } = renderHook(() =>
      useEnvGroupVarMutations("eg1", vi.fn().mockResolvedValue(undefined)),
    );

    await act(async () => {
      await result.current.setVar("TOKEN", "", true);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "eg1",
        key: "TOKEN",
        value: undefined,
        generateValue: true,
      },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Saved TOKEN");
  });

  it("mentions rollout only when an immediate write has linked services", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockImplementation(() => [mutate]);
    const { result } = renderHook(() =>
      useEnvGroupVarMutations(
        "eg1",
        vi.fn().mockResolvedValue(undefined),
        true,
      ),
    );

    await act(async () => {
      await result.current.setVar("TOKEN", "literal");
    });

    expect(toastSuccess).toHaveBeenCalledWith(
      "Saved TOKEN",
      expect.objectContaining({
        description: "Linked services are redeploying to apply the change.",
      }),
    );
  });
});

describe("useEnvGroupEnvironmentPatch", () => {
  it("pins the opaque revision and retries rollout with an empty patch", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValueOnce({
        data: {
          patchEnvGroupEnvironment: {
            revision: "egr1_next",
            affectedServiceIds: ["web"],
            failedServiceIds: ["web"],
          },
        },
      })
      .mockResolvedValueOnce({
        data: {
          patchEnvGroupEnvironment: {
            revision: "egr1_retry",
            affectedServiceIds: ["web"],
            failedServiceIds: [],
          },
        },
      });
    mockUseMutation.mockImplementation(() => [mutate, { loading: false }]);
    const refetch = vi.fn().mockResolvedValue(undefined);
    const { result } = renderHook(() =>
      useEnvGroupEnvironmentPatch("eg1", "egr1_initial", refetch),
    );

    await act(async () => {
      await result.current.save(
        {
          envVars: [{ key: "TOKEN", generateValue: true }],
          secretFiles: [{ name: "old.pem", delete: true }],
        },
        "rebuild",
      );
    });
    expect(mutate).toHaveBeenNthCalledWith(1, {
      variables: {
        id: "eg1",
        envVars: [{ key: "TOKEN", generateValue: true }],
        secretFiles: [{ name: "old.pem", delete: true }],
        saveMode: "rebuild",
        expectedRevision: "egr1_initial",
      },
    });

    await act(async () => {
      await expect(result.current.retryRollout("rebuild")).resolves.toBe(true);
    });
    expect(mutate).toHaveBeenNthCalledWith(2, {
      variables: {
        id: "eg1",
        envVars: [],
        secretFiles: [],
        saveMode: "rebuild",
        expectedRevision: "egr1_next",
      },
    });
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
