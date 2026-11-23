import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SandboxGroupsDocument } from "@/graphql/definitions";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";

const mutate = vi.fn();
const refetchQueries = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
  useApolloClient: () => ({ refetchQueries }),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

const workspace: { currentWorkspaceId: string | null } = {
  currentWorkspaceId: "tea-owner",
};
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => workspace,
}));

import { useSetSandboxGroupEnvironment } from "@/features/sandbox-groups/hooks/use-set-sandbox-group-environment";

const production = { id: "evm-production", name: "production" };

beforeEach(() => {
  mutate.mockReset();
  mutate.mockResolvedValue({ data: { setSandboxGroupEnvironment: {} } });
  refetchQueries.mockReset();
  refetchQueries.mockResolvedValue([]);
  toastSuccess.mockReset();
  toastError.mockReset();
  workspace.currentWorkspaceId = "tea-owner";
});

async function place(
  result: { current: ReturnType<typeof useSetSandboxGroupEnvironment> },
  moves: Parameters<
    ReturnType<typeof useSetSandboxGroupEnvironment>["setGroupsEnvironment"]
  >[1],
) {
  let moved = new Set<string>();
  await act(async () => {
    moved = await result.current.setGroupsEnvironment(production, moves);
  });
  return moved;
}

// w22/m603: Render's environment-resources move of a sandbox group.
describe("useSetSandboxGroupEnvironment", () => {
  it("puts a group in the environment and takes another out, in the selected workspace", async () => {
    const { result } = renderHook(() => useSetSandboxGroupEnvironment());

    const moved = await place(result, [
      { id: "sbg-in", name: "in", into: true },
      { id: "sbg-out", name: "out", into: false },
    ]);

    expect([...moved].sort()).toEqual(["sbg-in", "sbg-out"]);
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "sbg-in",
        environmentId: "evm-production",
        ownerId: "tea-owner",
      },
    });
    // A null environment takes the group out of the one it is in.
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "sbg-out", environmentId: null, ownerId: "tea-owner" },
    });
    expect(toastSuccess).toHaveBeenCalledOnce();
    expect(toastSuccess).toHaveBeenCalledWith(
      'Sandbox groups for "production" updated.',
    );
    // The groups list is re-read once for the whole batch.
    expect(refetchQueries).toHaveBeenCalledOnce();
    expect(refetchQueries).toHaveBeenCalledWith({
      include: [SandboxGroupsDocument],
    });
    expect(result.current.busyId).toBeNull();
  });

  it("resolves only the groups that moved, and says why the others did not", async () => {
    mutate.mockImplementation(async ({ variables }) => {
      if (variables.id === "sbg-locked")
        throw codedGraphQLError("PROTECTED_ENVIRONMENT_ADMIN_REQUIRED");
      if (variables.id === "sbg-gone")
        throw uncodedGraphQLError("sandbox group not found");
      return { data: { setSandboxGroupEnvironment: {} } };
    });
    const { result } = renderHook(() => useSetSandboxGroupEnvironment());

    const moved = await place(result, [
      { id: "sbg-ok", name: "ok", into: true },
      { id: "sbg-locked", name: "locked", into: true },
      { id: "sbg-gone", name: "gone", into: true },
    ]);

    expect([...moved]).toEqual(["sbg-ok"]);
    expect(toastError).toHaveBeenCalledWith(
      "Only an admin can create resources in a protected environment or move them into or out of it.",
    );
    expect(toastError).toHaveBeenCalledWith("Sandbox group not found");
    expect(toastSuccess).toHaveBeenCalledOnce();
  });

  it("falls back to its own words for a failure the server did not explain", async () => {
    mutate.mockRejectedValue(new Error("Failed to fetch"));
    const { result } = renderHook(() => useSetSandboxGroupEnvironment());

    const moved = await place(result, [
      { id: "sbg-a", name: "agents", into: true },
    ]);

    expect(moved.size).toBe(0);
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't move sandbox group agents. Please try again.",
    );
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(refetchQueries).not.toHaveBeenCalled();
  });

  it("keeps a move whose list re-read failed", async () => {
    refetchQueries.mockRejectedValue(new Error("refresh failed"));
    const { result } = renderHook(() => useSetSandboxGroupEnvironment());

    const moved = await place(result, [
      { id: "sbg-a", name: "agents", into: true },
    ]);

    expect([...moved]).toEqual(["sbg-a"]);
    expect(toastSuccess).toHaveBeenCalledOnce();
  });

  it("writes nothing for no moves", async () => {
    const { result } = renderHook(() => useSetSandboxGroupEnvironment());
    expect((await place(result, [])).size).toBe(0);
    expect(mutate).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
  });
});
