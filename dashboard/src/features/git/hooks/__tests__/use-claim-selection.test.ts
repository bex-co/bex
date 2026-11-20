import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useMutation: () => [vi.fn()],
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

import { useClaimSelection } from "@/features/git/hooks/use-claim-selection";

beforeEach(() => {
  mockUseQuery.mockReset();
});

// w1/m526 (ADR078 §3b): a chooser-mode selection may hold no candidates at
// all — the user administers no account with the App yet — and that is a live
// choice (offer the install), not an expired one.
describe("useClaimSelection", () => {
  it("reads a live selection with no candidates as empty, not gone", () => {
    mockUseQuery.mockReturnValue({
      data: { gitClaimSelection: { id: "gcs-1", candidates: [] } },
      loading: false,
    });

    const { result } = renderHook(() => useClaimSelection("gcs-1"));

    expect(result.current.gone).toBe(false);
    expect(result.current.candidates).toEqual([]);
  });

  it("reads a refused or expired selection as gone", () => {
    mockUseQuery.mockReturnValue({
      data: { gitClaimSelection: null },
      loading: false,
    });

    const { result } = renderHook(() => useClaimSelection("gcs-stale"));

    expect(result.current.gone).toBe(true);
  });

  it("carries each candidate's connected mark", () => {
    mockUseQuery.mockReturnValue({
      data: {
        gitClaimSelection: {
          id: "gcs-1",
          candidates: [
            { installationId: 1, accountLogin: "puncsky", connected: false },
            { installationId: 2, accountLogin: "bex-co", connected: true },
            { installationId: 3, accountLogin: "blockeden", connected: null },
          ],
        },
      },
      loading: false,
    });

    const { result } = renderHook(() => useClaimSelection("gcs-1"));

    expect(result.current.candidates).toEqual([
      { installationId: 1, accountLogin: "puncsky", connected: false },
      { installationId: 2, accountLogin: "bex-co", connected: true },
      { installationId: 3, accountLogin: "blockeden", connected: false },
    ]);
  });

  it("is neither gone nor loading without a selection id", () => {
    mockUseQuery.mockReturnValue({ data: undefined, loading: false });

    const { result } = renderHook(() => useClaimSelection(undefined));

    expect(result.current.gone).toBe(false);
    expect(mockUseQuery.mock.calls[0][1]).toMatchObject({ skip: true });
  });
});
