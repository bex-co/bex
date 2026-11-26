import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { WorkspaceLimitsDocument } from "@/graphql/definitions";
import {
  createErrorQueryResult,
  createLoadingQueryResult,
  createSuccessQueryResult,
} from "@/test/mocks/apollo";

// The setup file mocks the hook for every component test; this one reads the
// real hook.
vi.unmock("@/features/usage/hooks/use-pipeline-minutes-exceeded");
const { usePipelineMinutesExceeded } =
  await import("@/features/usage/hooks/use-pipeline-minutes-exceeded");

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

describe("usePipelineMinutesExceeded (w3/108)", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
    currentWorkspaceId = "tea-1";
  });

  it("is true only when bex-api says the current workspace's minutes are spent", () => {
    mockUseQuery.mockReturnValue(
      createSuccessQueryResult({
        workspaceLimits: { pipelineMinutesExceeded: true },
      }),
    );
    const { result } = renderHook(() => usePipelineMinutesExceeded());
    expect(result.current).toBe(true);
    // The Usage page's document and variables, so the two share one entry.
    const [document, options] = mockUseQuery.mock.calls[0];
    expect(document).toBe(WorkspaceLimitsDocument);
    expect(options).toMatchObject({
      variables: { ownerId: "tea-1" },
      skip: false,
    });
    expect(options).not.toHaveProperty("pollInterval");
  });

  it.each([
    [
      "minutes remain",
      createSuccessQueryResult({
        workspaceLimits: { pipelineMinutesExceeded: false },
      }),
    ],
    [
      "the answer is null",
      createSuccessQueryResult({
        workspaceLimits: { pipelineMinutesExceeded: null },
      }),
    ],
    ["the read is loading", createLoadingQueryResult()],
    ["the read failed", createErrorQueryResult("boom")],
  ])("is false when %s, leaving the server to refuse a build", (_, value) => {
    mockUseQuery.mockReturnValue(value);
    const { result } = renderHook(() => usePipelineMinutesExceeded());
    expect(result.current).toBe(false);
  });

  it("asks nothing before a workspace is chosen", () => {
    currentWorkspaceId = null;
    mockUseQuery.mockReturnValue(createLoadingQueryResult());
    const { result } = renderHook(() => usePipelineMinutesExceeded());
    expect(result.current).toBe(false);
    expect(mockUseQuery.mock.calls[0][1]).toMatchObject({ skip: true });
  });
});
