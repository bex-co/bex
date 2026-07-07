import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useOryFlow, clearStoredOryFlow } from "../use-ory-flow";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

const mockApi = {
  getLoginFlow: vi.fn(),
  getRegistrationFlow: vi.fn(),
  getRecoveryFlow: vi.fn(),
  getSettingsFlow: vi.fn(),
  createBrowserLoginFlow: vi.fn(),
  createBrowserRegistrationFlow: vi.fn(),
  createBrowserRecoveryFlow: vi.fn(),
  createBrowserSettingsFlow: vi.fn(),
};
vi.mock("@/common/lib/ory/frontend", () => ({
  createFrontendApi: () => mockApi,
}));

const LOGIN_KEY = "bex.ory-flow.login";

describe("useOryFlow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.sessionStorage.clear();
  });

  it("creates a fresh flow via AJAX and never touches the URL", async () => {
    const flow = { id: "fresh-1", ui: {} };
    mockApi.createBrowserLoginFlow.mockResolvedValue(flow);

    const { result } = renderHook(() => useOryFlow("login", undefined, "/"));

    await waitFor(() => expect(result.current).toBe(flow));
    expect(mockApi.createBrowserLoginFlow).toHaveBeenCalledWith({
      returnTo: new URL("/", window.location.origin).toString(),
    });
    expect(mockNavigate).not.toHaveBeenCalled();
    expect(window.sessionStorage.getItem(LOGIN_KEY)).toBe("fresh-1");
  });

  it("adopts an inbound ?flow= id and scrubs it from the URL", () => {
    const { result } = renderHook(() =>
      useOryFlow("login", "inbound-flow-id", "/"),
    );

    // The inbound id is persisted, the URL is replace-navigated clean, and
    // no fetch happens until the re-render without the param.
    expect(window.sessionStorage.getItem(LOGIN_KEY)).toBe("inbound-flow-id");
    expect(mockNavigate).toHaveBeenCalledWith(
      expect.objectContaining({ replace: true, to: "." }),
    );
    const searchUpdater = mockNavigate.mock.calls[0][0].search as (
      prev: Record<string, unknown>,
    ) => Record<string, unknown>;
    expect(searchUpdater({ flow: "inbound-flow-id", next: "/x" })).toEqual({
      flow: undefined,
      next: "/x",
    });
    expect(mockApi.getLoginFlow).not.toHaveBeenCalled();
    expect(mockApi.createBrowserLoginFlow).not.toHaveBeenCalled();
    expect(result.current).toBeNull();
  });

  it("resumes the sessionStorage flow after a reload", async () => {
    window.sessionStorage.setItem(LOGIN_KEY, "stored-1");
    const flow = { id: "stored-1", ui: {} };
    mockApi.getLoginFlow.mockResolvedValue(flow);

    const { result } = renderHook(() => useOryFlow("login", undefined, "/"));

    await waitFor(() => expect(result.current).toBe(flow));
    expect(mockApi.getLoginFlow).toHaveBeenCalledWith({ id: "stored-1" });
    expect(mockApi.createBrowserLoginFlow).not.toHaveBeenCalled();
  });

  it("self-heals a stale stored flow by minting a fresh one", async () => {
    window.sessionStorage.setItem(LOGIN_KEY, "expired-1");
    mockApi.getLoginFlow.mockRejectedValue(new Error("410 Gone"));
    const fresh = { id: "fresh-2", ui: {} };
    mockApi.createBrowserLoginFlow.mockResolvedValue(fresh);

    const { result } = renderHook(() => useOryFlow("login", undefined, "/"));

    await waitFor(() => expect(result.current).toBe(fresh));
    expect(window.sessionStorage.getItem(LOGIN_KEY)).toBe("fresh-2");
  });

  it("redirects to returnTo when a session already exists", async () => {
    mockApi.createBrowserLoginFlow.mockRejectedValue({
      response: new Response(
        JSON.stringify({ error: { id: "session_already_available" } }),
        { status: 400 },
      ),
    });

    renderHook(() => useOryFlow("login", undefined, "/deploys"));

    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/deploys",
        replace: true,
      }),
    );
  });

  it("clearStoredOryFlow drops the persisted id", () => {
    window.sessionStorage.setItem(LOGIN_KEY, "some-id");
    clearStoredOryFlow("login");
    expect(window.sessionStorage.getItem(LOGIN_KEY)).toBeNull();
  });
});
