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

// The hook holds two queries (w8/m153): the first review, and the second one a
// file whose resources already exist gets, which alone carries
// existingResources.
type Options = { skip: boolean; variables: Record<string, unknown> };
const isSecondReview = (options: Options) =>
  "existingResources" in options.variables;
const reviewOptions = (second: boolean): Options[] =>
  query.mock.calls
    .map(([, options]) => options as Options)
    .filter((options) => isSecondReview(options) === second);

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

// w8/250: a form that proposes a name asks the review about that name, and
// both reviews of a file that matches existing resources carry it.
it("sends the name a form proposes, and no name when it proposes none", () => {
  renderHook(() =>
    useBlueprintPreview(
      "https://github.com/a/app",
      "main",
      "render.yaml",
      undefined,
      "my-app",
    ),
  );
  expect(query).toHaveBeenCalledWith(
    BlueprintPreviewDocument,
    expect.objectContaining({
      variables: expect.objectContaining({ name: "my-app" }),
    }),
  );
  expect(
    query.mock.calls.every(
      ([, options]) => (options as Options).variables.name === "my-app",
    ),
  ).toBe(true);

  query.mockClear();
  renderHook(() =>
    useBlueprintPreview("https://github.com/a/app", "main", "render.yaml"),
  );
  expect(
    query.mock.calls.some(
      ([, options]) => "name" in (options as Options).variables,
    ),
  ).toBe(false);
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
  const lastOptions = () => reviewOptions(false).at(-1);
  const mount = (initial: Inputs) =>
    renderHook(
      ({ repo, branch, path }: Inputs) =>
        useBlueprintPreview(repo, branch, path),
      { initialProps: initial },
    );

  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it.each([
    ["the repository last", { repo: "", branch: "main", path: "render.yaml" }],
    [
      "the repository after the path",
      { repo: "", branch: "", path: "render.yaml" },
    ],
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
    expect(lastOptions()?.variables.path).toBe("infra/render.yaml");
  });

  it("is idle, with no preview and no request, while the inputs are incomplete", () => {
    const { result, rerender } = mount({ repo: "", branch: "", path: "" });
    rerender({ repo: "", branch: "main", path: "" });
    expect(result.current.loading).toBe(false);
    act(() => vi.advanceTimersByTime(400));
    expect(result.current.loading).toBe(false);
    expect(result.current.preview).toBeNull();
    expect(query.mock.calls.every(([, options]) => options.skip)).toBe(true);
  });

  // w4/532: with no branch the review reads the repository's default branch,
  // so a repo alone is a complete request, sent with no branch.
  it("previews a repository with no branch as its default branch", () => {
    const { rerender } = mount({ repo: "", branch: "", path: "" });
    rerender({ repo: REPO, branch: "", path: "" });
    act(() => vi.advanceTimersByTime(400));
    expect(lastOptions()).toMatchObject({
      skip: false,
      variables: expect.objectContaining({ repo: REPO, branch: null }),
    });
  });

  it("schedules nothing after unmount", () => {
    const { rerender, unmount } = mount({ repo: REPO, branch: "", path: "" });
    rerender({ repo: REPO, branch: "main", path: "" });
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});

// w8/m153, Render's creation flow: a file whose resources already exist in the
// workspace is reviewed again as a separate set under a name suffix, and that
// second review is the one the page shows and deploys.
describe("second review of a file that matches existing resources (w8/m153)", () => {
  const REPO = "https://github.com/a/app";
  const matched = [{ kind: "service", name: "api" }];
  const firstReview = {
    found: true,
    existingResources: "adopt",
    resourceSuffix: null,
    matchedResources: matched,
    validation: null,
  };
  const secondReview = {
    found: true,
    existingResources: "create_new",
    resourceSuffix: "ab12",
    // The second review lists the same matches; they must not ask for a third.
    matchedResources: matched,
    validation: null,
  };
  const idle = { data: undefined, loading: false, refetch: vi.fn() };
  const answer = (first: unknown, second: unknown) =>
    query.mockImplementation((_document: unknown, options: Options) =>
      isSecondReview(options) ? (options.skip ? idle : second) : first,
    );
  const mount = (blueprintId?: string) =>
    renderHook(() =>
      useBlueprintPreview(REPO, "main", "render.yaml", blueprintId),
    );

  it("asks for the second review with create_new and returns it", () => {
    answer(
      { data: { blueprintPreview: firstReview }, loading: false },
      { data: { blueprintPreview: secondReview }, loading: false },
    );
    const { result } = mount();

    expect(reviewOptions(true).at(-1)).toMatchObject({
      skip: false,
      variables: {
        repo: REPO,
        branch: "main",
        path: "render.yaml",
        ownerId: "tea-1",
        blueprintId: null,
        existingResources: "create_new",
      },
    });
    expect(result.current.loading).toBe(false);
    expect(result.current.preview).toMatchObject({
      existingResources: "create_new",
      resourceSuffix: "ab12",
      matchedResources: matched,
    });
    // Two queries a render, and the first never changes mode.
    expect(reviewOptions(false).length).toBe(reviewOptions(true).length);
    expect(reviewOptions(false).every((options) => !options.skip)).toBe(true);
  });

  it.each([
    ["is in flight", { data: undefined, loading: true }],
    // The render the second query starts on must not read as settled.
    ["has not answered", { data: undefined, loading: false }],
  ])("stays pending while the second review %s", (_label, second) => {
    answer(
      { data: { blueprintPreview: firstReview }, loading: false },
      { ...second, refetch: vi.fn() },
    );
    const { result } = mount();

    expect(result.current.loading).toBe(true);
    // Never the first review: its plan is the one that would change the
    // existing resources.
    expect(result.current.preview).toBeNull();
  });

  it("reports the second review's failure and retries that review", () => {
    const failure = new Error("network");
    const retry = vi.fn();
    answer(
      { data: { blueprintPreview: firstReview }, loading: false },
      { data: undefined, loading: false, error: failure, refetch: retry },
    );
    const { result } = mount();

    expect(result.current.loading).toBe(false);
    expect(result.current.preview).toBeNull();
    expect(result.current.error).toBe(failure);
    void result.current.refetch();
    expect(retry).toHaveBeenCalledOnce();
  });

  it("keeps the one review, with no existingResources, when nothing matches", () => {
    answer(
      {
        data: { blueprintPreview: { ...firstReview, matchedResources: [] } },
        loading: false,
      },
      { data: { blueprintPreview: secondReview }, loading: false },
    );
    const { result } = mount();

    expect(reviewOptions(true).every((options) => options.skip)).toBe(true);
    expect(reviewOptions(false).at(-1)?.variables).toEqual({
      repo: REPO,
      branch: "main",
      path: "render.yaml",
      ownerId: "tea-1",
      blueprintId: null,
    });
    expect(result.current.preview).toMatchObject({
      existingResources: "adopt",
      resourceSuffix: null,
      matchedResources: [],
    });
  });

  // bex-api refuses existingResources together with blueprintId.
  it("never asks for a second review of an existing Blueprint", () => {
    answer(
      { data: { blueprintPreview: firstReview }, loading: false },
      { data: { blueprintPreview: secondReview }, loading: false },
    );
    const { result } = mount("blp-1");

    expect(reviewOptions(true).every((options) => options.skip)).toBe(true);
    expect(result.current.preview?.existingResources).toBe("adopt");
  });
});
