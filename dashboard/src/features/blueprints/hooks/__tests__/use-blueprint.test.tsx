import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useQuery } from "@apollo/client/react";
import { useBlueprint } from "../use-blueprint";

vi.mock("@apollo/client/react", () => ({
  useQuery: vi.fn(),
}));

const refetch = vi.fn();

beforeEach(() => {
  vi.mocked(useQuery).mockReset();
  refetch.mockReset();
});

describe("useBlueprint", () => {
  // w4/m169: the id resolves its own workspace server-side; pinning the
  // selected workspace as ownerId made a link to another one 404.
  it("queries by id alone, never pinning the selected workspace", () => {
    vi.mocked(useQuery).mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch,
    } as never);

    renderHook(() => useBlueprint("blp-elsewhere"));

    const options = vi.mocked(useQuery).mock.calls[0][1] as {
      variables: Record<string, unknown>;
    };
    expect(options.variables).toEqual({ id: "blp-elsewhere" });
  });

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

  it("keeps a blueprint that has an id, normalized onto the view", () => {
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

    expect(result.current.blueprint).toEqual({
      ...blueprint,
      // Fields the row left null/absent are normalized to view defaults.
      path: "",
      autoSync: false,
      lastSync: null,
      resources: null,
    });
  });
});
