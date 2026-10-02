import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const query = vi.fn();
const client = { query };

vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => client,
}));

import { useWorkspaceEnvironmentIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";

beforeEach(() => {
  query.mockReset().mockImplementation(({ variables }) => {
    const suffix = variables.ownerId === "tea-a" ? "a" : "b";
    return Promise.resolve({
      data: {
        projects: [
          {
            id: `project-${suffix}`,
            name: `Project ${suffix}`,
            ownerId: variables.ownerId,
            serviceIds: [],
            databaseIds: [],
            keyValueIds: [],
          },
        ],
        workspaceEnvironments: [
          {
            id: `env-${suffix}`,
            projectId: `project-${suffix}`,
            name: `Environment ${suffix}`,
            ownerId: `tea-${suffix}`,
            createdAt: null,
            serviceIds: [`service-${suffix}`],
            databaseIds: [],
            keyValueIds: [],
            envGroupIds: [],
            protectedStatus: "unprotected",
            networkIsolationEnabled: false,
            ipAllowList: [],
            ipAllowListEntries: [],
          },
        ],
      },
    });
  });
});

describe("useWorkspaceEnvironmentIndex", () => {
  it("distinguishes an unknown workspace from a successfully empty index", async () => {
    query.mockResolvedValue({
      data: { projects: [], workspaceEnvironments: [] },
    });
    const { result, rerender } = renderHook(
      ({ ownerId }) => useWorkspaceEnvironmentIndex(ownerId),
      { initialProps: { ownerId: null as string | null } },
    );
    expect(result.current.ready).toBe(false);
    expect(query).not.toHaveBeenCalled();
    rerender({ ownerId: "tea-a" });
    expect(result.current.ready).toBe(false);
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.ready).toBe(true));
    expect(result.current.serviceEnvironmentById.size).toBe(0);
    expect(result.current.error).toBeUndefined();
  });

  it.each(["forbidden", "timeout"])(
    "keeps a %s lookup unresolved until a network retry succeeds",
    async (message) => {
      query.mockRejectedValueOnce(new Error(message));
      const { result } = renderHook(() =>
        useWorkspaceEnvironmentIndex("tea-a"),
      );
      await waitFor(() => expect(result.current.error?.message).toBe(message));
      expect(result.current.ready).toBe(false);
      expect(result.current.loading).toBe(false);

      act(() => result.current.retry());
      expect(result.current.loading).toBe(true);
      expect(result.current.ready).toBe(false);
      await waitFor(() => expect(result.current.ready).toBe(true));
      expect(result.current.serviceEnvironmentById.get("service-a")).toBe(
        "env-a",
      );
      expect(result.current.error).toBeUndefined();
      expect(query).toHaveBeenLastCalledWith(
        expect.objectContaining({ fetchPolicy: "network-only" }),
      );
    },
  );

  it("does not interpret incomplete scope data as confirmed Workspace", async () => {
    query.mockResolvedValueOnce({
      data: { projects: [], workspaceEnvironments: null },
    });
    const { result } = renderHook(() => useWorkspaceEnvironmentIndex("tea-a"));
    await waitFor(() => expect(result.current.error).toBeDefined());
    expect(result.current.ready).toBe(false);
  });

  it("preserves the resolved index and reports an error when a refresh fails", async () => {
    const { result } = renderHook(() => useWorkspaceEnvironmentIndex("tea-a"));
    await waitFor(() => expect(result.current.ready).toBe(true));
    const membership = result.current.serviceEnvironmentById;
    query.mockRejectedValueOnce(new Error("unavailable"));
    act(() => result.current.retry());
    expect(result.current.ready).toBe(true);
    expect(result.current.loading).toBe(true);
    expect(result.current.serviceEnvironmentById).toBe(membership);
    await waitFor(() =>
      expect(result.current.error?.message).toBe("unavailable"),
    );
    expect(result.current.ready).toBe(true);
    expect(result.current.serviceEnvironmentById).toBe(membership);
  });

  it("rebuilds scope and service membership when the workspace changes", async () => {
    const { result, rerender } = renderHook(
      ({ ownerId }) => useWorkspaceEnvironmentIndex(ownerId),
      { initialProps: { ownerId: "tea-a" as string | null } },
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.byId.get("env-a")?.name).toBe("Environment a");
    expect(result.current.serviceEnvironmentById.get("service-a")).toBe(
      "env-a",
    );

    rerender({ ownerId: "tea-b" });
    expect(result.current.ready).toBe(false);
    expect(result.current.byId.size).toBe(0);
    expect(result.current.serviceEnvironmentById.size).toBe(0);
    await waitFor(() => expect(result.current.byId.has("env-b")).toBe(true));
    expect(result.current.byId.has("env-a")).toBe(false);
    expect(result.current.serviceEnvironmentById.has("service-a")).toBe(false);
    expect(query).toHaveBeenCalledTimes(2);
  });

  it("ignores a late error from the previous workspace", async () => {
    let rejectPrevious!: (error: Error) => void;
    query.mockImplementationOnce(
      () =>
        new Promise((_, reject) => {
          rejectPrevious = reject;
        }),
    );
    const { result, rerender } = renderHook(
      ({ ownerId }) => useWorkspaceEnvironmentIndex(ownerId),
      { initialProps: { ownerId: "tea-a" } },
    );
    rerender({ ownerId: "tea-b" });
    await waitFor(() => expect(result.current.ready).toBe(true));
    await act(async () => rejectPrevious(new Error("expired request")));
    expect(result.current.byId.has("env-b")).toBe(true);
    expect(result.current.byId.has("env-a")).toBe(false);
    expect(result.current.error).toBeUndefined();
  });
});
