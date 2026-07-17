import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

import { useEnvironments } from "@/features/environments/hooks/use-environments";

/** The `projectId` the hook most recently asked Apollo to query with. */
function lastProjectId(): unknown {
  const calls = mockUseQuery.mock.calls;
  const [, options] = calls[calls.length - 1] as [
    unknown,
    { variables: { projectId: string | null } },
  ];
  return options.variables.projectId;
}

function lastSkip(): boolean {
  const calls = mockUseQuery.mock.calls;
  const [, options] = calls[calls.length - 1] as [unknown, { skip: boolean }];
  return options.skip;
}

beforeEach(() => {
  mockUseQuery.mockReset();
});

describe("useEnvironments", () => {
  it("maps wire environments onto normalized views and drops nulls", () => {
    mockUseQuery.mockReturnValue({
      data: {
        environments: [
          {
            __typename: "Environment",
            id: "env-1",
            projectId: "prj-1",
            name: "staging",
            ownerId: "tea-1",
            createdAt: "2026-01-01T00:00:00Z",
            serviceIds: ["svc-a", null, "svc-b"],
            databaseIds: ["db-a", null],
            keyValueIds: [null, "kv-a"],
            envGroupIds: ["evg-a", null],
          },
          null,
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvironments("prj-1"));

    expect(result.current.environments).toHaveLength(1);
    expect(result.current.environments[0]).toMatchObject({
      id: "env-1",
      projectId: "prj-1",
      name: "staging",
      ownerId: "tea-1",
      // null ids are filtered out of the normalized view, for every list.
      serviceIds: ["svc-a", "svc-b"],
      databaseIds: ["db-a"],
      keyValueIds: ["kv-a"],
      envGroupIds: ["evg-a"],
      // w6/m19: absent ACL fields default to Render's own defaults.
      protectedStatus: "unprotected",
      networkIsolationEnabled: false,
      ipAllowListEntries: [],
    });
  });

  it("maps w6/m19 protected-environment ACL fields when present", () => {
    mockUseQuery.mockReturnValue({
      data: {
        environments: [
          {
            __typename: "Environment",
            id: "env-1",
            projectId: "prj-1",
            name: "production",
            ownerId: "tea-1",
            createdAt: "2026-01-01T00:00:00Z",
            serviceIds: ["svc-a"],
            protectedStatus: "protected",
            networkIsolationEnabled: true,
            ipAllowList: ["198.51.100.0/24"],
            ipAllowListEntries: [
              {
                __typename: "IPAllowListEntry",
                cidrBlock: "10.0.0.0/24",
                description: "office",
              },
              {
                __typename: "IPAllowListEntry",
                cidrBlock: "192.168.0.0/16",
                description: null,
              },
              null,
            ],
          },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvironments("prj-1"));

    expect(result.current.environments[0]).toMatchObject({
      protectedStatus: "protected",
      networkIsolationEnabled: true,
      // Object-form entries take precedence over the legacy string list;
      // null descriptions normalize to the editor's optional empty value.
      ipAllowListEntries: [
        { cidrBlock: "10.0.0.0/24", description: "office" },
        { cidrBlock: "192.168.0.0/16", description: "" },
      ],
    });
  });

  it("maps legacy plain-string rules when entries are unavailable", () => {
    mockUseQuery.mockReturnValue({
      data: {
        environments: [
          {
            __typename: "Environment",
            id: "env-1",
            projectId: "prj-1",
            name: "legacy",
            ownerId: "tea-1",
            ipAllowList: ["10.0.0.0/8", null],
            ipAllowListEntries: null,
          },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvironments("prj-1"));

    expect(result.current.environments[0].ipAllowListEntries).toEqual([
      { cidrBlock: "10.0.0.0/8", description: "" },
    ]);
  });

  it("returns an empty list (not a crash) when data is undefined", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvironments("prj-1"));

    expect(result.current.environments).toEqual([]);
    expect(result.current.loading).toBe(true);
  });

  it("queries with the given projectId and does not skip", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });

    renderHook(() => useEnvironments("prj-42"));

    expect(lastProjectId()).toBe("prj-42");
    expect(lastSkip()).toBe(false);
  });

  it("skips the query (and stays loading) until a projectId resolves", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useEnvironments(null));

    expect(lastSkip()).toBe(true);
    // Mirrors useProjects/useServices: no flash of the empty state before the
    // id lands.
    expect(result.current.loading).toBe(true);
    expect(result.current.environments).toEqual([]);
  });
});
