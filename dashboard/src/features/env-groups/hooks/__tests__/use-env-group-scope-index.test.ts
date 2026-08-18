import { renderHook, waitFor } from "@testing-library/react";
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
    await waitFor(() => expect(result.current.byId.has("env-b")).toBe(true));
    expect(result.current.byId.has("env-a")).toBe(false);
    expect(result.current.serviceEnvironmentById.has("service-a")).toBe(false);
    expect(query).toHaveBeenCalledTimes(2);
  });
});
