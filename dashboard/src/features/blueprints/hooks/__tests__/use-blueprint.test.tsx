import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useQuery } from "@apollo/client/react";
import { useBlueprint } from "../use-blueprint";

vi.mock("@apollo/client/react", () => ({
  useQuery: vi.fn(),
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

const refetch = vi.fn();

beforeEach(() => {
  vi.mocked(useQuery).mockReset();
  refetch.mockReset();
});

describe("useBlueprint", () => {
  it("treats an empty adapter object as not found", () => {
    vi.mocked(useQuery).mockReturnValue({
      data: { blueprint: {} },
      loading: false,
      error: undefined,
      refetch,
    } as never);

    const { result } = renderHook(() => useBlueprint("missing"));

    expect(result.current.blueprint).toBeNull();
  });

  it("keeps a blueprint that has an id", () => {
    const blueprint = {
      id: "blp-test",
      name: "Example",
      repo: "owner/repo",
      branch: "main",
      manifest: "services: []",
      status: "synced",
      createdAt: null,
      updatedAt: null,
    };
    vi.mocked(useQuery).mockReturnValue({
      data: { blueprint },
      loading: false,
      error: undefined,
      refetch,
    } as never);

    const { result } = renderHook(() => useBlueprint("blp-test"));

    expect(result.current.blueprint).toBe(blueprint);
  });
});
