import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const setCurrentWorkspaceId = vi.fn();
let state: {
  currentWorkspaceId: string | null;
  workspaces: { id: string }[];
  loading: boolean;
};
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ ...state, setCurrentWorkspaceId }),
}));

import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";

const A = "tea-aaaaaaaaaaaaaaaaaaaa";
const B = "tea-bbbbbbbbbbbbbbbbbbbb";
const FOREIGN = "tea-cccccccccccccccccccc";

beforeEach(() => {
  setCurrentWorkspaceId.mockReset();
  state = {
    currentWorkspaceId: A,
    workspaces: [{ id: A }, { id: B }],
    loading: false,
  };
});

const render = (resourceId: string, ownerId: string | null) =>
  renderHook(({ r, o }) => useAdoptResourceWorkspace(r, o), {
    initialProps: { r: resourceId, o: ownerId },
  });

describe("useAdoptResourceWorkspace (w6/091)", () => {
  it("selects the owning workspace when another one is selected", () => {
    render("srv-1", B);
    expect(setCurrentWorkspaceId).toHaveBeenCalledExactlyOnceWith(B);
  });

  it("leaves the selection alone when it already owns the resource", () => {
    render("srv-1", A);
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
  });

  it("never selects a workspace the caller is not a member of", () => {
    render("srv-1", FOREIGN);
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
  });

  it("waits for the owner and the membership list", () => {
    state.loading = true;
    const { rerender } = render("srv-1", null);
    rerender({ r: "srv-1", o: B });
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    state.loading = false;
    rerender({ r: "srv-1", o: B });
    expect(setCurrentWorkspaceId).toHaveBeenCalledExactlyOnceWith(B);
  });

  it("does not undo a deliberate switch made from the same page", () => {
    const { rerender } = render("srv-1", B);
    state.currentWorkspaceId = B;
    rerender({ r: "srv-1", o: B });
    // The user picks A in the switcher while the page is still mounted.
    state.currentWorkspaceId = A;
    rerender({ r: "srv-1", o: B });
    expect(setCurrentWorkspaceId).toHaveBeenCalledExactlyOnceWith(B);
  });

  it("adopts again for the next resource", () => {
    const { rerender } = render("srv-1", B);
    state.currentWorkspaceId = B;
    rerender({ r: "srv-2", o: A });
    expect(setCurrentWorkspaceId.mock.calls).toEqual([[B], [A]]);
  });
});
