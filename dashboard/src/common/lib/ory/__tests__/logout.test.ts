import { beforeEach, describe, expect, it, vi } from "vitest";
import { endBrowserSession } from "../logout";

const { createBrowserLogoutFlow, clearStore, invalidateSessionCache } =
  vi.hoisted(() => ({
    createBrowserLogoutFlow: vi.fn(),
    clearStore: vi.fn(),
    invalidateSessionCache: vi.fn(),
  }));

vi.mock("@/common/lib/ory/frontend", () => ({
  createFrontendApi: () => ({ createBrowserLogoutFlow }),
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

    expect(fetch).toHaveBeenCalledWith("https://auth.example/logout?token=abc", {
      credentials: "include",
    });
    expect(invalidateSessionCache).toHaveBeenCalledOnce();
    expect(clearStore).toHaveBeenCalledOnce();
  });

  it("does not clear caches when the provider logout request fails", async () => {
    createBrowserLogoutFlow.mockResolvedValue({
      logout_url: "https://auth.example/logout?token=abc",
    });
    vi.mocked(fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(endBrowserSession()).rejects.toThrow(
      "logout request failed: 500",
    );

    expect(invalidateSessionCache).not.toHaveBeenCalled();
    expect(clearStore).not.toHaveBeenCalled();
  });
});
