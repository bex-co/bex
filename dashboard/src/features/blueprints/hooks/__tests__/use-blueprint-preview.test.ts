import { renderHook } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { BlueprintPreviewDocument } from "@/graphql/definitions";
import { useBlueprintPreview } from "../use-blueprint-preview";

const query = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => query(...args),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

beforeEach(() => {
  query.mockReset();
  query.mockReturnValue({ data: undefined, loading: false, refetch: vi.fn() });
});

it("scopes an existing blueprint preview so its detached resources can be planned", () => {
  renderHook(() =>
    useBlueprintPreview(
      "https://github.com/a/app",
      "main",
      "render.yaml",
      "blp-1",
    ),
  );
  expect(query).toHaveBeenCalledWith(
    BlueprintPreviewDocument,
    expect.objectContaining({
      variables: {
        repo: "https://github.com/a/app",
        branch: "main",
        path: "render.yaml",
        ownerId: "tea-1",
        blueprintId: "blp-1",
      },
    }),
  );
});

it("keeps create previews unscoped to an existing blueprint", () => {
  renderHook(() =>
    useBlueprintPreview("https://github.com/a/app", "main", "render.yaml"),
  );
  expect(query).toHaveBeenCalledWith(
    BlueprintPreviewDocument,
    expect.objectContaining({
      variables: expect.objectContaining({ blueprintId: null }),
    }),
  );
});
