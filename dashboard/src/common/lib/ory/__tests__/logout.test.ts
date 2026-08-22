import { beforeEach, describe, expect, it, vi } from "vitest";
import { endBrowserSession } from "../logout";

const { createBrowserLogoutFlow, toSession, clearStore, invalidateSessionCache } =
  vi.hoisted(() => ({
    createBrowserLogoutFlow: vi.fn(),
    toSession: vi.fn(),
    clearStore: vi.fn(),
    invalidateSessionCache: vi.fn(),
  }));

vi.mock("@/common/lib/ory/frontend", () => ({
  createFrontendApi: () => ({ createBrowserLogoutFlow, toSession }),
}));

vi.mock("@/common/apollo/client", () => ({
  getClient: () => ({ clearStore }),
}));

vi.mock("@/common/server-fn/session", () => ({
  invalidateSessionCache,
}));

describe("endBrowserSession", () => {
  beforeEach(() => {
    createBrowserLogoutFlow.mockReset();
    toSession.mockReset();
    clearStore.mockReset();
    invalidateSessionCache.mockReset();
    vi.stubGlobal("fetch", vi.fn());
  });

  it("fetches the Kratos logout URL then drops cached session data", async () => {
    createBrowserLogoutFlow.mockResolvedValue({
      logout_url: "https://auth.example/logout?token=abc",
    });
    vi.mocked(fetch).mockResolvedValue(new Response(null, { status: 204 }));
    clearStore.mockResolvedValue(undefined);

    await endBrowserSession();

    expect(createBrowserLogoutFlow).toHaveBeenCalledWith({
      returnTo: `${window.location.origin}/`,
    });
    expect(fetch).toHaveBeenCalledWith("https://auth.example/logout?token=abc", {
      credentials: "include",
    });
    expect(invalidateSessionCache).toHaveBeenCalledOnce();
    expect(clearStore).toHaveBeenCalledOnce();
  });

  it("does not clear caches when the provider logout request fails with 5xx", async () => {
    createBrowserLogoutFlow.mockResolvedValue({
      logout_url: "https://auth.example/logout?token=abc",
    });
    vi.mocked(fetch).mockResolvedValue(new Response(null, { status: 500 }));
    // Session still alive — this is a real provider failure, keep the retry UI.
    toSession.mockResolvedValue({ id: "session-1" });

    await expect(endBrowserSession()).rejects.toThrow(
      "logout request failed: 500",
    );

    expect(invalidateSessionCache).not.toHaveBeenCalled();
    expect(clearStore).not.toHaveBeenCalled();
  });

  it("treats a 401 from createBrowserLogoutFlow as already signed out", async () => {
    createBrowserLogoutFlow.mockRejectedValue({
      response: new Response(null, { status: 401 }),
    });
    clearStore.mockResolvedValue(undefined);

    await endBrowserSession();

    expect(fetch).not.toHaveBeenCalled();
    expect(invalidateSessionCache).toHaveBeenCalledOnce();
    expect(clearStore).toHaveBeenCalledOnce();
  });

  it("treats a 401 from the logout URL fetch as already signed out", async () => {
    createBrowserLogoutFlow.mockResolvedValue({
      logout_url: "https://auth.example/logout?token=abc",
    });
    vi.mocked(fetch).mockResolvedValue(new Response(null, { status: 401 }));
    clearStore.mockResolvedValue(undefined);

    await endBrowserSession();

    expect(invalidateSessionCache).toHaveBeenCalledOnce();
    expect(clearStore).toHaveBeenCalledOnce();
  });

  it("confirms via whoami when the logout fetch fails on the return_to redirect", async () => {
    // Kratos deletes the session then 303s to return_to (the dashboard
    // origin); fetch's CORS mode rejects that redirect even though the
    // logout succeeded. whoami 401 proves the session is gone.
    createBrowserLogoutFlow.mockResolvedValue({
      logout_url: "https://auth.example/logout?token=abc",
    });
    vi.mocked(fetch).mockRejectedValue(new TypeError("Failed to fetch"));
    toSession.mockRejectedValue({ response: new Response(null, { status: 401 }) });
    clearStore.mockResolvedValue(undefined);

    await endBrowserSession();

    expect(toSession).toHaveBeenCalledOnce();
    expect(invalidateSessionCache).toHaveBeenCalledOnce();
    expect(clearStore).toHaveBeenCalledOnce();
  });

  it("rethrows when the logout fetch fails and the session is still alive", async () => {
    createBrowserLogoutFlow.mockResolvedValue({
      logout_url: "https://auth.example/logout?token=abc",
    });
    const networkError = new TypeError("Failed to fetch");
    vi.mocked(fetch).mockRejectedValue(networkError);
    toSession.mockResolvedValue({ id: "session-1" });

    await expect(endBrowserSession()).rejects.toThrow(networkError);

    expect(invalidateSessionCache).not.toHaveBeenCalled();
    expect(clearStore).not.toHaveBeenCalled();
  });
});
