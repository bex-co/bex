import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";

const { mockNavigate, mockUseWorkspaces, cookieFns } = vi.hoisted(() => ({
  mockNavigate: vi.fn(),
  mockUseWorkspaces: vi.fn(),
  cookieFns: {
    get: vi.fn(),
    set: vi.fn(),
    remove: vi.fn(),
  },
}));

vi.mock("@/features/workspaces/hooks/use-workspaces", () => ({
  useWorkspaces: (...args: unknown[]) => mockUseWorkspaces(...args),
}));
vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});
vi.mock("@/common/hooks/use-cookie-storage-state/cookie", () => ({
  getCookie: cookieFns.get,
  setCookie: cookieFns.set,
  removeCookie: cookieFns.remove,
}));

import { WorkspaceProvider } from "@/features/workspaces/context";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  getPersistedWorkspaceId,
  WORKSPACE_SELECTION_KEY,
} from "@/features/workspaces/lib/selection";

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

let persistedCookie: string | undefined;

beforeEach(() => {
  mockUseWorkspaces.mockReset();
  mockNavigate.mockReset();
  vi.mocked(localStorage.getItem).mockReset();
  vi.mocked(localStorage.setItem).mockReset();
  persistedCookie = undefined;
  cookieFns.get.mockReset();
  cookieFns.get.mockImplementation(() => persistedCookie);
  cookieFns.set.mockReset();
  cookieFns.set.mockImplementation((_key: string, value: string) => {
    persistedCookie = value;
  });
  cookieFns.remove.mockReset();
  cookieFns.remove.mockImplementation(() => {
    persistedCookie = undefined;
  });
});

function mockReady(workspaces = WORKSPACES, extra: Record<string, unknown> = {}) {
  mockUseWorkspaces.mockReturnValue({
    workspaces,
    loading: false,
    error: undefined,
    ready: true,
    refetch: vi.fn(),
    ...extra,
  });
}

describe("WorkspaceProvider", () => {
  it("falls back to the first workspace when nothing is stored", async () => {
    mockReady();
    const { result } = renderHook(() => useWorkspace(), {
      wrapper: WorkspaceProvider,
    });

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );
    expect(result.current.currentWorkspace?.name).toBe("acme-hq");
    expect(getPersistedWorkspaceId()).toBe("tea-1");
    expect(cookieFns.set).toHaveBeenCalledWith(
      WORKSPACE_SELECTION_KEY,
      "tea-1",
      expect.objectContaining({ path: "/", sameSite: "lax" }),
    );
    expect(localStorage.getItem).not.toHaveBeenCalled();
    expect(localStorage.setItem).not.toHaveBeenCalled();
  });

  it("restores the cookie-backed server selection when it still exists", async () => {
    mockReady();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider initialWorkspaceId="tea-2">
        {children}
      </WorkspaceProvider>
    );
    const { result } = renderHook(() => useWorkspace(), { wrapper });

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-2"),
    );
    expect(localStorage.getItem).not.toHaveBeenCalled();
  });

  it("keeps the server-selected workspace stable through hydration", async () => {
    mockReady();
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
    expect(localStorage.getItem).not.toHaveBeenCalled();
  });

  it("falls back to the first workspace when the persisted selection was deleted", async () => {
    mockReady();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider initialWorkspaceId="tea-deleted">
        {children}
      </WorkspaceProvider>
    );
    const { result } = renderHook(() => useWorkspace(), { wrapper });

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );
    expect(getPersistedWorkspaceId()).toBe("tea-1");
  });

  it("clears selection and routes to /new/workspace on successful empty membership", async () => {
    mockReady();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider initialWorkspaceId="tea-1">
        {children}
      </WorkspaceProvider>
    );
    const { result, rerender } = renderHook(() => useWorkspace(), { wrapper });
    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );

    mockReady([], {});
    rerender();
    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBeNull(),
    );
    expect(cookieFns.remove).toHaveBeenCalled();
    expect(mockNavigate).toHaveBeenCalledWith(
      expect.objectContaining({
        to: "/new/workspace",
        replace: true,
      }),
    );
  });

  it("does not clear selection on membership transport errors", async () => {
    mockReady();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider initialWorkspaceId="tea-1">
        {children}
      </WorkspaceProvider>
    );
    const { result, rerender } = renderHook(() => useWorkspace(), { wrapper });
    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );

    mockUseWorkspaces.mockReturnValue({
      workspaces: [],
      loading: false,
      error: new Error("network"),
      ready: false,
      refetch: vi.fn(),
    });
    rerender();
    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("persists switching in the cookie and restores it after a hard reload", async () => {
    mockReady();
    const firstWrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider initialWorkspaceId="tea-1">
        {children}
      </WorkspaceProvider>
    );
    const { result, unmount } = renderHook(() => useWorkspace(), {
      wrapper: firstWrapper,
    });
    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-1"),
    );

    act(() => result.current.setCurrentWorkspaceId("tea-2"));

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-2"),
    );
    expect(getPersistedWorkspaceId()).toBe("tea-2");
    expect(cookieFns.set).toHaveBeenCalledWith(
      WORKSPACE_SELECTION_KEY,
      "tea-2",
      expect.objectContaining({ path: "/", sameSite: "lax" }),
    );
    expect(localStorage.getItem).not.toHaveBeenCalled();
    expect(localStorage.setItem).not.toHaveBeenCalled();

    unmount();
    const persistedId = getPersistedWorkspaceId();
    const reloadWrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider initialWorkspaceId={persistedId}>
        {children}
      </WorkspaceProvider>
    );
    const reloaded = renderHook(() => useWorkspace(), {
      wrapper: reloadWrapper,
    });
    await waitFor(() =>
      expect(reloaded.result.current.currentWorkspaceId).toBe("tea-2"),
    );
  });
});
