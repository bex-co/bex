import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
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

import { useCreateSandboxGroup } from "@/features/sandbox-groups/hooks/use-create-sandbox-group";

const GROUP = {
  __typename: "SandboxGroup",
  id: "sbg-created",
  name: "ci",
  region: "frankfurt",
  isDefault: true,
  environmentId: null,
  concurrencyLimit: 1000,
};

beforeEach(() => {
  mutate.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  workspace.currentWorkspaceId = "tea-owner";
});

async function createCi(
  result: { current: ReturnType<typeof useCreateSandboxGroup> },
  environmentId?: string | null,
) {
  let group: Awaited<ReturnType<typeof result.current.create>> = null;
  await act(async () => {
    group = await result.current.create("ci", environmentId);
  });
  return group;
}

describe("useCreateSandboxGroup", () => {
  it("creates the group in the selected workspace, leaving the region to bex-api, and returns it", async () => {
    mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
    const { result } = renderHook(() => useCreateSandboxGroup());

    expect(await createCi(result)).toEqual(GROUP);

    expect(mutate).toHaveBeenCalledWith({
      variables: { name: "ci", ownerId: "tea-owner" },
    });
    expect(result.current.refusal).toBeNull();
    expect(toastError).not.toHaveBeenCalled();
  });

  it("treats a response without an id as a failure", async () => {
    mutate.mockResolvedValue({ data: { createSandboxGroup: null } });
    const { result } = renderHook(() => useCreateSandboxGroup());

    expect(await createCi(result)).toBeNull();
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't create sandbox group ci. Please try again.",
    );
  });

  it("sends nothing before the workspace list resolves", async () => {
    workspace.currentWorkspaceId = null;
    const { result } = renderHook(() => useCreateSandboxGroup());

    expect(await createCi(result)).toBeNull();
    expect(mutate).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalled();
  });

  it("shows a duplicate name inline, decided by the code, not the wording", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("SANDBOX_GROUP_NAME_TAKEN", { field: "name" }),
    );
    const { result } = renderHook(() => useCreateSandboxGroup());

    expect(await createCi(result)).toBeNull();
    expect(result.current.refusal).toEqual({
      kind: "name",
      message:
        'A sandbox group named "ci" already exists in this workspace. Choose a different name.',
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("shows the group limit inline with the number the server names", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("sandbox_group_limit_reached", { limit: 10 }),
    );
    const { result } = renderHook(() => useCreateSandboxGroup());

    expect(await createCi(result)).toBeNull();
    expect(result.current.refusal).toEqual({
      kind: "limit",
      message: "This workspace already has the maximum of 10 sandbox groups.",
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("singularizes a limit of one and words a limit with no number", async () => {
    mutate.mockRejectedValueOnce(
      codedGraphQLError("sandbox_group_limit_reached", { limit: 1 }),
    );
    const { result } = renderHook(() => useCreateSandboxGroup());
    await createCi(result);
    expect(result.current.refusal?.message).toBe(
      "This workspace already has the maximum of 1 sandbox group.",
    );

    mutate.mockRejectedValueOnce(
      codedGraphQLError("sandbox_group_limit_reached"),
    );
    await createCi(result);
    expect(result.current.refusal?.message).toBe(
      "This workspace has reached its sandbox group limit.",
    );
  });

  it("shows a refused name inline with the server's maximum length", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("SANDBOX_GROUP_NAME_INVALID", {
        field: "name",
        maxLength: 100,
      }),
    );
    const { result } = renderHook(() => useCreateSandboxGroup());

    await createCi(result);
    expect(result.current.refusal).toEqual({
      kind: "name",
      message:
        "A group name must be 1 to 100 characters, with no leading or trailing space and no control characters.",
    });
  });

  it("clears an inline refusal on request and on the next attempt", async () => {
    mutate.mockRejectedValueOnce(
      codedGraphQLError("SANDBOX_GROUP_NAME_TAKEN", { field: "name" }),
    );
    const { result } = renderHook(() => useCreateSandboxGroup());
    await createCi(result);
    expect(result.current.refusal).not.toBeNull();

    act(() => result.current.clearRefusal());
    expect(result.current.refusal).toBeNull();

    mutate.mockRejectedValueOnce(
      codedGraphQLError("SANDBOX_GROUP_NAME_TAKEN", { field: "name" }),
    );
    await createCi(result);
    expect(result.current.refusal).not.toBeNull();

    mutate.mockResolvedValueOnce({ data: { createSandboxGroup: GROUP } });
    await createCi(result);
    expect(result.current.refusal).toBeNull();
  });

  it("toasts the server's own reason for any other refusal", async () => {
    mutate.mockRejectedValue(
      uncodedGraphQLError("sandbox groups need the control-plane store"),
    );
    const { result } = renderHook(() => useCreateSandboxGroup());

    await createCi(result);
    expect(result.current.refusal).toBeNull();
    expect(toastError).toHaveBeenCalledWith(
      "Sandbox groups need the control-plane store",
    );
  });

  it("falls back to the generic toast for a transport failure", async () => {
    mutate.mockRejectedValue(new Error("Failed to fetch"));
    const { result } = renderHook(() => useCreateSandboxGroup());

    await createCi(result);
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't create sandbox group ci. Please try again.",
    );
  });

  // w22/m412: an optional project environment, as Render's dashboard offers.
  it("places the group in the chosen environment by sending its id", async () => {
    mutate.mockResolvedValue({
      data: { createSandboxGroup: { ...GROUP, environmentId: "evm-prod" } },
    });
    const { result } = renderHook(() => useCreateSandboxGroup());

    expect(await createCi(result, "evm-prod")).toEqual({
      ...GROUP,
      environmentId: "evm-prod",
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        name: "ci",
        ownerId: "tea-owner",
        environmentId: "evm-prod",
      },
    });
  });

  it.each([
    ["no environment", undefined],
    ["a null environment", null],
    ["an empty one", ""],
  ])("sends no environmentId for %s", async (_label, environmentId) => {
    mutate.mockResolvedValue({ data: { createSandboxGroup: GROUP } });
    const { result } = renderHook(() => useCreateSandboxGroup());

    await createCi(result, environmentId);

    // The key is absent, not null: an official Render body never names it.
    expect(mutate.mock.calls[0]?.[0]).toStrictEqual({
      variables: { name: "ci", ownerId: "tea-owner" },
    });
  });

  it("shows a protected environment's admin-only refusal inline, decided by the code", async () => {
    mutate.mockRejectedValue(
      codedGraphQLError("PROTECTED_ENVIRONMENT_ADMIN_REQUIRED", {
        environmentId: "evm-prod",
      }),
    );
    const { result } = renderHook(() => useCreateSandboxGroup());

    expect(await createCi(result, "evm-prod")).toBeNull();
    expect(result.current.refusal).toEqual({
      kind: "environment",
      message:
        "Only an admin can create resources in a protected environment or move them into or out of it.",
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("says an environment deleted in the meantime is gone, only when one was chosen", async () => {
    mutate.mockRejectedValue(codedGraphQLError("NOT_FOUND"));
    const { result } = renderHook(() => useCreateSandboxGroup());

    expect(await createCi(result, "evm-gone")).toBeNull();
    expect(result.current.refusal).toEqual({
      kind: "environment",
      message: "That environment no longer exists. Choose another or none.",
    });
    expect(toastError).not.toHaveBeenCalled();

    // With none chosen a not-found cannot be about an environment: it is
    // reported as any other failure, never worded as one.
    await createCi(result);
    expect(result.current.refusal).toBeNull();
    expect(toastError).toHaveBeenCalledTimes(1);
  });
});
