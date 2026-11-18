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
  const actual = await vi.importActual<typeof import("@tanstack/react-router")>(
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
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import {
  getPersistedWorkspaceId,
  getPersistedWorkspaceShape,
  WORKSPACE_SELECTION_KEY,
  WORKSPACE_SHAPE_KEY,
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

const cookies = new Map<string, string>();

beforeEach(() => {
  mockUseWorkspaces.mockReset();
  mockNavigate.mockReset();
  vi.mocked(localStorage.getItem).mockReset();
  vi.mocked(localStorage.setItem).mockReset();
  cookies.clear();
  cookieFns.get.mockReset();
  cookieFns.get.mockImplementation((key: string) => cookies.get(key));
  cookieFns.set.mockReset();
  cookieFns.set.mockImplementation((key: string, value: string) => {
    cookies.set(key, value);
  });
  cookieFns.remove.mockReset();
  cookieFns.remove.mockImplementation((key: string) => {
    cookies.delete(key);
  });
});

function mockReady(
  workspaces = WORKSPACES,
  extra: Record<string, unknown> = {},
) {
  mockUseWorkspaces.mockReturnValue({
    workspaces,
    loading: false,
    error: undefined,
    ready: true,
    verified: true,
    refetch: vi.fn(),
    ...extra,
  });
}

describe("WorkspaceProvider", () => {
  it("has no current workspace without a verified session, whatever the cookie says (w6/078, w6/079)", () => {
    mockUseWorkspaces.mockReturnValue({
      workspaces: [],
      loading: false,
      error: undefined,
      ready: false,
      verified: false,
      refetch: vi.fn(),
    });
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider initialWorkspaceId="tea-1">
        {children}
      </WorkspaceProvider>
    );
    const { result } = renderHook(() => useWorkspace(), { wrapper });
    expect(result.current.currentWorkspaceId).toBeNull();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

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

  // w4/640: a resource page deep-linked with no cookie adopts its own
  // workspace in a child effect, which runs before the provider's fallback in
  // the same commit; the fallback must not overwrite it with the first one.
  it("keeps a first visit's adopted workspace in the cookie, not the fallback", async () => {
    mockReady();
    function Adopter() {
      useAdoptResourceWorkspace("srv-1", "tea-2");
      return null;
    }
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider>
        <Adopter />
        {children}
      </WorkspaceProvider>
    );
    const { result } = renderHook(() => useWorkspace(), { wrapper });

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-2"),
    );
    expect(getPersistedWorkspaceId()).toBe("tea-2");
    expect(cookieFns.set).not.toHaveBeenCalledWith(
      WORKSPACE_SELECTION_KEY,
      "tea-1",
      expect.anything(),
    );
  });

  it("still moves an existing cookie for another workspace to the adopted one", async () => {
    mockReady();
    cookies.set(WORKSPACE_SELECTION_KEY, "tea-1");
    function Adopter() {
      useAdoptResourceWorkspace("srv-1", "tea-2");
      return null;
    }
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <WorkspaceProvider initialWorkspaceId="tea-1">
        <Adopter />
        {children}
      </WorkspaceProvider>
    );
    const { result } = renderHook(() => useWorkspace(), { wrapper });

    await waitFor(() =>
      expect(result.current.currentWorkspaceId).toBe("tea-2"),
    );
    expect(getPersistedWorkspaceId()).toBe("tea-2");
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
    await waitFor(() => expect(result.current.currentWorkspaceId).toBeNull());
    expect(cookies.has(WORKSPACE_SELECTION_KEY)).toBe(false);
    expect(cookies.has(WORKSPACE_SHAPE_KEY)).toBe(false);
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
      verified: true,
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
  // w3/073: a direct load's pending frames have only the selection cookie
  // until the list arrives, so the provider stores the selected workspace's
  // shape beside it whenever the list is known.
  it("stores the selected workspace's shape beside the selection, and follows a switch", async () => {
    const org = {
      id: "org-1",
      name: "Acme",
      primaryWorkspaceId: "tea-1",
      requireTwoFactor: false,
      requireGoogleLogin: true,
    };
    const listed = [
      { ...WORKSPACES[0], plan: "scale", organization: org, primary: true },
      { ...WORKSPACES[1], organization: null, primary: false },
    ];
    mockReady(listed);
    const { result } = renderHook(() => useWorkspace(), {
      wrapper: WorkspaceProvider,
    });

    await waitFor(() =>
      expect(getPersistedWorkspaceShape("tea-1")).toEqual({
        role: "admin",
        plan: "scale",
        organization: org,
        planOwner: null,
        otherWorkspaces: true,
      }),
    );
    expect(cookieFns.set).toHaveBeenCalledWith(
      WORKSPACE_SHAPE_KEY,
      expect.any(String),
      expect.objectContaining({ path: "/", sameSite: "lax" }),
    );

    act(() => result.current.setCurrentWorkspaceId("tea-2"));

    await waitFor(() =>
      expect(getPersistedWorkspaceShape("tea-2")).toEqual({
        role: "admin",
        plan: "pro",
        organization: null,
        planOwner: null,
        otherWorkspaces: true,
      }),
    );
    expect(getPersistedWorkspaceShape("tea-1")).toBeNull();
  });

  it("stores no shape while the list is unconfirmed", () => {
    mockUseWorkspaces.mockReturnValue({
      workspaces: WORKSPACES,
      loading: false,
      error: new Error("network"),
      ready: false,
      verified: true,
      refetch: vi.fn(),
    });
    renderHook(() => useWorkspace(), {
      wrapper: ({ children }: { children: React.ReactNode }) => (
        <WorkspaceProvider initialWorkspaceId="tea-1">
          {children}
        </WorkspaceProvider>
      ),
    });

    expect(cookies.has(WORKSPACE_SHAPE_KEY)).toBe(false);
  });
});
