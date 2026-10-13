import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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

// w4/282: the query stays skipped for one debounce after the inputs become
// complete. Reporting "not loading" for that interval let the create page
// enable Deploy before any preview had been requested.
describe("pending state across the input debounce (w4/282)", () => {
  const REPO = "https://github.com/a/app";
  type Inputs = { repo: string; branch: string; path: string };
  const lastOptions = () => query.mock.calls.at(-1)?.[1];
  const mount = (initial: Inputs) =>
    renderHook(
      ({ repo, branch, path }: Inputs) =>
        useBlueprintPreview(repo, branch, path),
      { initialProps: initial },
    );

  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it.each([
    ["the branch last", { repo: REPO, branch: "", path: "render.yaml" }],
    ["the repository last", { repo: "", branch: "main", path: "render.yaml" }],
  ])("is pending from the render that completes %s", (_label, initial) => {
    const { result, rerender } = mount(initial);
    expect(result.current.loading).toBe(false);

    rerender({ repo: REPO, branch: "main", path: "render.yaml" });
    expect(result.current.loading).toBe(true);
    expect(result.current.preview).toBeNull();
    // Nothing is requested before the debounce has passed.
    act(() => vi.advanceTimersByTime(399));
    expect(lastOptions()).toMatchObject({ skip: true });
    expect(result.current.loading).toBe(true);

    query.mockReturnValue({ data: undefined, loading: true, refetch: vi.fn() });
    act(() => vi.advanceTimersByTime(1));
    expect(lastOptions()).toMatchObject({
      skip: false,
      variables: expect.objectContaining({ repo: REPO, branch: "main" }),
    });
    expect(result.current.loading).toBe(true);

    query.mockReturnValue({
      data: { blueprintPreview: null },
      loading: false,
      refetch: vi.fn(),
    });
    rerender({ repo: REPO, branch: "main", path: "render.yaml" });
    expect(result.current.loading).toBe(false);
  });

  it("stays pending through a later path edit until the new inputs are queried", () => {
    const { result, rerender } = mount({
      repo: REPO,
      branch: "main",
      path: "render.yaml",
    });
    expect(result.current.loading).toBe(false);
    rerender({ repo: REPO, branch: "main", path: "infra/render.yaml" });
    expect(result.current.loading).toBe(true);
    act(() => vi.advanceTimersByTime(400));
    expect(result.current.loading).toBe(false);
    expect(lastOptions().variables.path).toBe("infra/render.yaml");
  });

  it("is idle, with no preview and no request, while the inputs are incomplete", () => {
    const { result, rerender } = mount({ repo: "", branch: "", path: "" });
    rerender({ repo: REPO, branch: "", path: "" });
    expect(result.current.loading).toBe(false);
    act(() => vi.advanceTimersByTime(400));
    expect(result.current.loading).toBe(false);
    expect(result.current.preview).toBeNull();
    expect(query.mock.calls.every(([, options]) => options.skip)).toBe(true);
  });

  it("schedules nothing after unmount", () => {
    const { rerender, unmount } = mount({ repo: REPO, branch: "", path: "" });
    rerender({ repo: REPO, branch: "main", path: "" });
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
