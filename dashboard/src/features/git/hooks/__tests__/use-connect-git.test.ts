import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { error: (...a: unknown[]) => toastError(...a), success: vi.fn() },
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

import {
  useConnectGit,
  useConnectGitAccount,
} from "@/features/git/hooks/use-connect-git";

const INSTALL_URL = "https://github.com/apps/bex/installations/new?state=abc";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastError.mockReset();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useConnectGit", () => {
  it("newTab mode opens the install in a new tab and leaves the current page", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { connectGit: { installUrl: INSTALL_URL } } });
    mockUseMutation.mockReturnValue([mutate]);
    const fakeTab = { location: { href: "" }, close: vi.fn() };
    const openMock = vi.fn().mockReturnValue(fakeTab);
    vi.stubGlobal("open", openMock);
    const location = { href: "" };
    vi.stubGlobal("location", location);

    const { result } = renderHook(() => useConnectGit({ newTab: true }));
    await act(async () => {
      await result.current.connect();
    });

    // Tab is pre-opened synchronously (popup-blocker safe), then navigated.
    expect(openMock).toHaveBeenCalledWith("", "_blank");
    expect(fakeTab.location.href).toBe(INSTALL_URL);
    // The create page itself is never navigated away.
    expect(location.href).toBe("");
    expect(mutate).toHaveBeenCalledWith({ variables: { ownerId: "tea-1" } });
  });

  it("same-tab mode (default) navigates the current page", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { connectGit: { installUrl: INSTALL_URL } } });
    mockUseMutation.mockReturnValue([mutate]);
    const openMock = vi.fn();
    vi.stubGlobal("open", openMock);
    const location = { href: "" };
    vi.stubGlobal("location", location);

    const { result } = renderHook(() => useConnectGit());
    await act(async () => {
      await result.current.connect();
    });

    expect(openMock).not.toHaveBeenCalled();
    expect(location.href).toBe(INSTALL_URL);
  });

  it("closes the pre-opened tab and toasts when the mutation fails", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("boom"));
    mockUseMutation.mockReturnValue([mutate]);
    const fakeTab = { location: { href: "" }, close: vi.fn() };
    vi.stubGlobal("open", vi.fn().mockReturnValue(fakeTab));
    vi.stubGlobal("location", { href: "" });

    const { result } = renderHook(() => useConnectGit({ newTab: true }));
    await act(async () => {
      await result.current.connect();
    });

    expect(fakeTab.close).toHaveBeenCalled();
    expect(toastError).toHaveBeenCalled();
  });
});

// w1/m526 (ADR078 §3b): the single "Connect GitHub account" entry starts the
// OAuth claim in chooser mode for the SELECTED workspace — the one flow GitHub
// carries the signed state through for an account that already has the App.
describe("useConnectGitAccount", () => {
  const CLAIM_URL =
    "https://github.com/login/oauth/authorize?client_id=bex&state=abc";

  it("starts the chooser-mode claim for the selected workspace", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { claimGit: { claimUrl: CLAIM_URL } } });
    mockUseMutation.mockReturnValue([mutate]);
    const location = { href: "" };
    vi.stubGlobal("location", location);
    vi.stubGlobal("open", vi.fn());

    const { result } = renderHook(() => useConnectGitAccount());
    await act(async () => {
      await result.current.connect();
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { ownerId: "tea-1", chooser: true },
    });
    // Same tab by default: the Settings card, where the callback lands.
    expect(location.href).toBe(CLAIM_URL);
  });

  it("newTab mode runs the round trip in a pre-opened tab", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { claimGit: { claimUrl: CLAIM_URL } } });
    mockUseMutation.mockReturnValue([mutate]);
    const fakeTab = { location: { href: "" }, close: vi.fn() };
    const openMock = vi.fn().mockReturnValue(fakeTab);
    vi.stubGlobal("open", openMock);
    const location = { href: "" };
    vi.stubGlobal("location", location);

    const { result } = renderHook(() => useConnectGitAccount({ newTab: true }));
    await act(async () => {
      await result.current.connect();
    });

    expect(openMock).toHaveBeenCalledWith("", "_blank");
    expect(fakeTab.location.href).toBe(CLAIM_URL);
    // The page holding the unsaved input is never navigated away.
    expect(location.href).toBe("");
  });

  it("closes the pre-opened tab and toasts when no URL comes back", async () => {
    const mutate = vi.fn().mockResolvedValue({ data: { claimGit: null } });
    mockUseMutation.mockReturnValue([mutate]);
    const fakeTab = { location: { href: "" }, close: vi.fn() };
    vi.stubGlobal("open", vi.fn().mockReturnValue(fakeTab));
    vi.stubGlobal("location", { href: "" });

    const { result } = renderHook(() => useConnectGitAccount({ newTab: true }));
    await act(async () => {
      await result.current.connect();
    });

    expect(fakeTab.close).toHaveBeenCalled();
    expect(toastError).toHaveBeenCalled();
    expect(result.current.busy).toBe(false);
  });
});
