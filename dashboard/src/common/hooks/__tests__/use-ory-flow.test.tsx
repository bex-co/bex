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

    const { result } = renderHook(() => useOryFlow("login", undefined));

    await waitFor(() => expect(result.current).toBe(flow));
    expect(mockApi.createBrowserLoginFlow).toHaveBeenCalledWith({
      returnTo: new URL("/", window.location.origin).toString(),
    });
    expect(mockNavigate).not.toHaveBeenCalled();
    expect(window.sessionStorage.getItem(LOGIN_KEY)).toBe("fresh-1");
  });

  it("adopts an inbound ?flow= id and scrubs it from the URL", () => {
    const { result } = renderHook(() => useOryFlow("login", "inbound-flow-id"));

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

    const { result } = renderHook(() => useOryFlow("login", undefined));

    await waitFor(() => expect(result.current).toBe(flow));
    expect(mockApi.getLoginFlow).toHaveBeenCalledWith({ id: "stored-1" });
    expect(mockApi.createBrowserLoginFlow).not.toHaveBeenCalled();
  });

  it("self-heals a stale stored flow by minting a fresh one", async () => {
    window.sessionStorage.setItem(LOGIN_KEY, "expired-1");
    mockApi.getLoginFlow.mockRejectedValue(new Error("410 Gone"));
    const fresh = { id: "fresh-2", ui: {} };
    mockApi.createBrowserLoginFlow.mockResolvedValue(fresh);

    const { result } = renderHook(() => useOryFlow("login", undefined));

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

    renderHook(() => useOryFlow("login", undefined, { returnTo: "/deploys" }));

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

  // --- OAuth2 login_challenge passthrough (w4/m9) ---

  it("binds the flow to a login_challenge, skipping any stored flow and not persisting", async () => {
    window.sessionStorage.setItem(LOGIN_KEY, "stored-1"); // must be ignored
    const flow = { id: "oauth-flow", ui: {} };
    mockApi.createBrowserLoginFlow.mockResolvedValue(flow);

    const { result } = renderHook(() =>
      useOryFlow("login", undefined, { loginChallenge: "challenge-123" }),
    );

    await waitFor(() => expect(result.current).toBe(flow));
    expect(mockApi.getLoginFlow).not.toHaveBeenCalled(); // stored flow not linked to Hydra
    expect(mockApi.createBrowserLoginFlow).toHaveBeenCalledWith({
      returnTo: new URL("/", window.location.origin).toString(),
      loginChallenge: "challenge-123",
    });
    // OAuth-linked flows are never persisted for later ordinary visits.
    expect(window.sessionStorage.getItem(LOGIN_KEY)).toBe("stored-1");
  });

  it("follows redirect_browser_to when an existing session satisfies the challenge", async () => {
    mockApi.createBrowserLoginFlow.mockRejectedValue({
      response: new Response(
        JSON.stringify({
          error: { id: "browser_location_change_required" },
          redirect_browser_to: "https://oauth.bex.co/continue",
        }),
        { status: 422 },
      ),
    });
    const original = window.location;
    Object.defineProperty(window, "location", {
      value: { ...original, href: original.href },
      writable: true,
    });

    renderHook(() =>
      useOryFlow("login", undefined, { loginChallenge: "challenge-123" }),
    );

    await waitFor(() =>
      expect(window.location.href).toBe("https://oauth.bex.co/continue"),
    );
    Object.defineProperty(window, "location", { value: original });
  });

  it("degrades to the ordinary login page on a stale challenge", async () => {
    const fresh = { id: "plain-flow", ui: {} };
    mockApi.createBrowserLoginFlow
      .mockRejectedValueOnce({
        response: new Response(
          JSON.stringify({ error: { id: "bad_request" } }),
          { status: 400 },
        ),
      })
      .mockResolvedValueOnce(fresh);

    const { result } = renderHook(() =>
      useOryFlow("login", undefined, { loginChallenge: "stale-challenge" }),
    );

    await waitFor(() => expect(result.current).toBe(fresh));
    // second call retried without the challenge
    const secondCall = mockApi.createBrowserLoginFlow.mock.calls[1][0];
    expect(secondCall.loginChallenge).toBeUndefined();
  });

  // --- aal2 second-factor step-up (w4/m11 MFA) ---

  const sessionAlreadyAvailable = () => ({
    response: new Response(
      JSON.stringify({ error: { id: "session_already_available" } }),
      { status: 400 },
    ),
  });

  it.each(["totp", "webauthn", "lookup_secret"])(
    "mints an aal2 step-up flow and renders the %s challenge when a second factor is owed",
    async (group) => {
      // The auth guard bounced an aal1-session user here (whoami 403'd under
      // `highest_available`), so the first-factor flow is refused and we must
      // present the second-factor challenge instead of navigating away.
      const stepUp = {
        id: "aal2-flow",
        ui: { nodes: [{ group, attributes: {} }] },
      };
      mockApi.createBrowserLoginFlow
        .mockRejectedValueOnce(sessionAlreadyAvailable())
        .mockResolvedValueOnce(stepUp);

      const { result } = renderHook(() =>
        useOryFlow("login", undefined, { returnTo: "/deploys" }),
      );

      await waitFor(() => expect(result.current).toBe(stepUp));
      // the retry requested aal2...
      expect(mockApi.createBrowserLoginFlow).toHaveBeenLastCalledWith(
        expect.objectContaining({ aal: "aal2" }),
      );
      // ...and we render the challenge rather than bouncing to returnTo.
      expect(mockNavigate).not.toHaveBeenCalled();
      // step-up flows are bound to the live session and never persisted.
      expect(window.sessionStorage.getItem(LOGIN_KEY)).toBeNull();
    },
  );

  it("navigates on when the aal2 step-up flow carries no second factor", async () => {
    // A fully-aal2 session (or an identity with no second factor) manually
    // visiting /auth/login: the step-up flow has nothing to challenge, so we
    // must send the user on rather than render an empty challenge card.
    const emptyFlow = {
      id: "aal2-empty",
      ui: { nodes: [{ group: "default", attributes: {} }] },
    };
    mockApi.createBrowserLoginFlow
      .mockRejectedValueOnce(sessionAlreadyAvailable())
      .mockResolvedValueOnce(emptyFlow);

    const { result } = renderHook(() =>
      useOryFlow("login", undefined, { returnTo: "/home" }),
    );

    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/home",
        replace: true,
      }),
    );
    expect(result.current).toBeNull();
  });

  it("navigates on when the aal2 step-up itself is refused", async () => {
    // Kratos rejects the aal2 request outright (e.g. no second factor to
    // satisfy it) — the catch must fall through to navigating on, never leave
    // the user stranded on a blank page.
    mockApi.createBrowserLoginFlow
      .mockRejectedValueOnce(sessionAlreadyAvailable())
      .mockRejectedValueOnce(new Error("aal2 not possible"));

    renderHook(() => useOryFlow("login", undefined, { returnTo: "/x" }));

    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({ to: "/x", replace: true }),
    );
  });

  it("does not attempt an aal2 step-up for non-login flows", async () => {
    // settings/registration/recovery never step up — a session_already_available
    // there is not a second-factor situation.
    mockApi.createBrowserSettingsFlow.mockRejectedValue(
      sessionAlreadyAvailable(),
    );

    renderHook(() => useOryFlow("settings", undefined, { returnTo: "/y" }));

    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({ to: "/y", replace: true }),
    );
    // only the single settings-flow attempt; no aal2 login retry.
    expect(mockApi.createBrowserLoginFlow).not.toHaveBeenCalled();
  });
});
