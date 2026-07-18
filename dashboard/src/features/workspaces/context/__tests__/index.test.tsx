import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";

const mockUseWorkspaces = vi.fn();
vi.mock("@/features/workspaces/hooks/use-workspaces", () => ({
  useWorkspaces: (...args: unknown[]) => mockUseWorkspaces(...args),
}));

import { WorkspaceProvider } from "@/features/workspaces/context";
import { useWorkspace } from "@/features/workspaces/context/hooks";

const WORKSPACES = [
  {
    id: "tea-1",
    name: "acme-hq",
    plan: "hobby",
    role: "admin",
    createdAt: null,
  },
  {
    id: "tea-2",
    name: "acme-staging",
    plan: "pro",
    role: "admin",
    createdAt: null,
  },
];

beforeEach(() => {
  mockUseWorkspaces.mockReset();
  vi.mocked(localStorage.getItem).mockReset();
  vi.mocked(localStorage.setItem).mockReset();
});

describe("WorkspaceProvider", () => {
  it("falls back to the first workspace when nothing is stored", async () => {
    mockUseWorkspaces.mockReturnValue({
      workspaces: WORKSPACES,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    vi.mocked(localStorage.getItem).mockReturnValue(null);

    const { result } = renderHook(() => useWorkspace(), {
      wrapper: WorkspaceProvider,
    });

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );
    expect(result.current.currentWorkspace?.name).toBe("acme-hq");
  });

  it("restores the persisted selection when it still exists", async () => {
    mockUseWorkspaces.mockReturnValue({
      workspaces: WORKSPACES,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    vi.mocked(localStorage.getItem).mockReturnValue("tea-2");

    const { result } = renderHook(() => useWorkspace(), {
      wrapper: WorkspaceProvider,
    });

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-2"),
    );
  });

  it("keeps the server-selected workspace stable through hydration", async () => {
    mockUseWorkspaces.mockReturnValue({
      workspaces: WORKSPACES,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    vi.mocked(localStorage.getItem).mockReturnValue("tea-2");

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider initialWorkspaceId="tea-1">
        {children}
      </WorkspaceProvider>
    );
    const { result } = renderHook(() => useWorkspace(), { wrapper });

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );
    expect(result.current.currentWorkspace?.name).toBe("acme-hq");
  });

  it("falls back to the first workspace when the persisted selection was deleted", async () => {
    mockUseWorkspaces.mockReturnValue({
      workspaces: WORKSPACES,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    vi.mocked(localStorage.getItem).mockReturnValue("tea-deleted");

    const { result } = renderHook(() => useWorkspace(), {
      wrapper: WorkspaceProvider,
    });

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );
  });

  it("setCurrentWorkspaceId persists the selection and updates every consumer", async () => {
    mockUseWorkspaces.mockReturnValue({
      workspaces: WORKSPACES,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    vi.mocked(localStorage.getItem).mockReturnValue("tea-1");

    const { result } = renderHook(() => useWorkspace(), {
      wrapper: WorkspaceProvider,
    });
    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );

    act(() => result.current.setCurrentWorkspaceId("tea-2"));

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-2"),
    );
    expect(localStorage.setItem).toHaveBeenCalledWith(
      "bex.selectedWorkspaceId",
      "tea-2",
    );
  });
});
