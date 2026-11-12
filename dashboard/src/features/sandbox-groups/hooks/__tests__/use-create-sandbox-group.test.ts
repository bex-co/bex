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
  concurrencyLimit: 1000,
};

beforeEach(() => {
  mutate.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  workspace.currentWorkspaceId = "tea-owner";
});

async function createCi(result: {
  current: ReturnType<typeof useCreateSandboxGroup>;
}) {
  let group: Awaited<ReturnType<typeof result.current.create>> = null;
  await act(async () => {
    group = await result.current.create("ci");
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
});
