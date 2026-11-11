import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { toast } from "sonner";
import { useCreateBlueprint } from "@/features/blueprints/hooks/use-create-blueprint";

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const REPO = "https://github.com/a/app";

beforeEach(() => {
  vi.mocked(toast.error).mockClear();
  mutate.mockReset();
  mutate.mockResolvedValue({
    data: { createBlueprint: { id: "exs-1", name: "app" } },
  });
});

const variables = () =>
  mutate.mock.calls[0][0].variables as Record<string, unknown>;

// w8/m153: a Blueprint whose file matches existing resources is created as a
// separate set under the suffix its preview reviewed; any other create names
// neither, and bex-api manages the names the file gives.
describe("useCreateBlueprint existingResources (w8/m153)", () => {
  it("sends create_new with the reviewed suffix", async () => {
    const { result } = renderHook(() => useCreateBlueprint());

    let outcome;
    await act(async () => {
      outcome = await result.current.create(
        REPO,
        "main",
        "render.yaml",
        "app",
        undefined,
        [],
        { existingResources: "create_new", resourceSuffix: "ab12" },
      );
    });

    expect(variables()).toMatchObject({
      repo: REPO,
      branch: "main",
      path: "render.yaml",
      name: "app",
      ownerId: "tea-1",
      existingResources: "create_new",
      resourceSuffix: "ab12",
    });
    expect(outcome).toMatchObject({
      status: "success",
      blueprint: { id: "exs-1" },
    });
  });

  it("sends neither for a Blueprint that manages the names its file gives", async () => {
    const { result } = renderHook(() => useCreateBlueprint());

    await act(async () => {
      await result.current.create(REPO, "main", "render.yaml", "app");
    });

    expect(variables().existingResources).toBeUndefined();
    expect(variables().resourceSuffix).toBeUndefined();
  });

  it("lets bex-api choose the suffix when the review carried none", async () => {
    const { result } = renderHook(() => useCreateBlueprint());

    await act(async () => {
      await result.current.create(
        REPO,
        "main",
        "render.yaml",
        "app",
        undefined,
        [],
        {
          existingResources: "create_new",
          resourceSuffix: null,
        },
      );
    });

    expect(variables().existingResources).toBe("create_new");
    expect(variables().resourceSuffix).toBeUndefined();
  });
});

// w8/250: the name rule is said at the name field, not in a toast.
describe("useCreateBlueprint name refusal (w8/250)", () => {
  const nameTaken = (name: string) =>
    new CombinedGraphQLErrors({
      errors: [
        {
          message: `a Blueprint named "${name}" already exists in this workspace`,
          extensions: { code: "BLUEPRINT_NAME_TAKEN", name },
        },
      ],
    });

  it("answers name_taken with the refused name and shows no toast", async () => {
    mutate.mockRejectedValue(nameTaken("app"));
    const { result } = renderHook(() => useCreateBlueprint());

    let outcome;
    await act(async () => {
      outcome = await result.current.create(REPO, "main", "render.yaml", "App");
    });

    expect(outcome).toEqual({ status: "name_taken", name: "app" });
    expect(toast.error).not.toHaveBeenCalled();
  });
});
