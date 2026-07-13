import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { KRATOS_PUBLIC_URL } from "@/common/lib/ory/config";
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

/** Where the hook sends the browser when it hands a flow back to Kratos. */
const kratosBrowserUrl = (params: Record<string, string>) =>
  `${KRATOS_PUBLIC_URL}/self-service/login/browser?${new URLSearchParams(params)}`;

const RETURN_TO_ROOT = new URL("/", window.location.origin).toString();

/**
 * Full-page navigations (`window.location.href = …`) are how the hook leaves for
 * Kratos or Hydra; jsdom won't follow one, so stub location to make it readable.
 */
let realLocation: Location;
function watchLocation() {
  realLocation = window.location;
  Object.defineProperty(window, "location", {
    value: { ...realLocation, href: realLocation.href },
    writable: true,
    configurable: true,
  });
}

describe("useOryFlow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.sessionStorage.clear();
    watchLocation();
  });

  afterEach(() => {
    Object.defineProperty(window, "location", {
      value: realLocation,
      configurable: true,
    });
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
        to: "/",
        href: "/deploys",
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

    renderHook(() =>
      useOryFlow("login", undefined, { loginChallenge: "challenge-123" }),
    );

    await waitFor(() =>
      expect(window.location.href).toBe("https://oauth.bex.co/continue"),
    );
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

  // --- a signed-in browser continuing an OAuth2 authorization (w4/m17) ---

  it("hands a satisfied login_challenge back to Kratos when it answers `200 null`", async () => {
    // The most common connect-an-agent path: the user is already signed into
    // the dashboard. Kratos (v1.3.1) accepts the login challenge against that
    // session and answers the AJAX call with HTTP 200 and a body of literally
    // `null` — no flow, no redirect, nothing to render. Re-asking as a *browser*
    // is what moves the authorization on (Kratos 303s to Hydra's continue URL);
    // rendering the null would leave the login page on its skeleton forever.
    mockApi.createBrowserLoginFlow.mockResolvedValue(null);

    const { result } = renderHook(() =>
      useOryFlow("login", undefined, { loginChallenge: "challenge-123" }),
    );

    await waitFor(() =>
      expect(window.location.href).toBe(
        kratosBrowserUrl({
          return_to: RETURN_TO_ROOT,
          login_challenge: "challenge-123",
        }),
      ),
    );
    expect(result.current).toBeNull(); // never rendered as a flow
  });

  it("hands the challenge back to Kratos when it refuses the flow outright", async () => {
    // The same short-circuit, spelled as an error rather than a null body: a
    // live session refuses a login flow. The challenge must ride along — bouncing
    // to returnTo here would silently abandon the authorization.
    mockApi.createBrowserLoginFlow.mockRejectedValue(sessionAlreadyAvailable());

    renderHook(() =>
      useOryFlow("login", undefined, {
        returnTo: "/deploys",
        loginChallenge: "challenge-123",
      }),
    );

    await waitFor(() =>
      expect(window.location.href).toBe(
        kratosBrowserUrl({
          return_to: new URL("/deploys", window.location.origin).toString(),
          login_challenge: "challenge-123",
        }),
      ),
    );
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("keeps the login_challenge on the fallback bootstrap after an unknown failure", async () => {
    // Any unhandled Kratos failure falls back to its redirect-based bootstrap.
    // That bootstrap used to drop the challenge, quietly abandoning the
    // authorization the user was in the middle of.
    mockApi.createBrowserLoginFlow.mockRejectedValue(new Error("kratos is down"));

    renderHook(() =>
      useOryFlow("login", undefined, { loginChallenge: "challenge-123" }),
    );

    await waitFor(() =>
      expect(window.location.href).toBe(
        kratosBrowserUrl({
          return_to: RETURN_TO_ROOT,
          login_challenge: "challenge-123",
        }),
      ),
    );
  });

  // --- aal2 second-factor step-up (w4/m11 MFA, lifted in w4/m17) ---

  const sessionAlreadyAvailable = () => ({
    response: new Response(
      JSON.stringify({ error: { id: "session_already_available" } }),
      { status: 400 },
    ),
  });

  it.each(["totp", "webauthn", "lookup_secret"])(
    "renders the %s challenge directly when the guard says a second factor is owed",
    async (group) => {
      // `requireAuth` read `session_aal2_required` off the session fetch and sent
      // the user here with `aal=aal2` — so the step-up flow is the *first* thing
      // asked of Kratos. No trial first-factor flow, no probing (w4/m17).
      const stepUp = {
        id: "aal2-flow",
        requested_aal: "aal2",
        ui: { nodes: [{ group, attributes: {} }] },
      };
      mockApi.createBrowserLoginFlow.mockResolvedValue(stepUp);

      const { result } = renderHook(() =>
        useOryFlow("login", undefined, { returnTo: "/deploys", aal: "aal2" }),
      );

      await waitFor(() => expect(result.current).toBe(stepUp));
      expect(mockApi.createBrowserLoginFlow).toHaveBeenCalledTimes(1);
      expect(mockApi.createBrowserLoginFlow).toHaveBeenCalledWith(
        expect.objectContaining({ aal: "aal2" }),
      );
      // The challenge renders rather than bouncing to returnTo...
      expect(mockNavigate).not.toHaveBeenCalled();
      // ...and step-up flows, bound to the live session, are never persisted.
      expect(window.sessionStorage.getItem(LOGIN_KEY)).toBeNull();
    },
  );

  it("ignores a stored first-factor flow when asked to step up", async () => {
    window.sessionStorage.setItem(LOGIN_KEY, "stored-1"); // must not be resumed
    const stepUp = {
      id: "aal2-flow",
      requested_aal: "aal2",
      ui: { nodes: [{ group: "totp", attributes: {} }] },
    };
    mockApi.createBrowserLoginFlow.mockResolvedValue(stepUp);

    const { result } = renderHook(() =>
      useOryFlow("login", undefined, { aal: "aal2" }),
    );

    await waitFor(() => expect(result.current).toBe(stepUp));
    expect(mockApi.getLoginFlow).not.toHaveBeenCalled();
  });

  it("navigates on when the aal2 flow carries no second factor", async () => {
    // An identity with no second factor, reached only by hand-typing
    // `?aal=aal2`: Kratos still hands back a flow, but its only node is the CSRF
    // token. Rendering that empty challenge card is worse than moving on.
    const emptyFlow = {
      id: "aal2-empty",
      requested_aal: "aal2",
      ui: { nodes: [{ group: "default", attributes: {} }] },
    };
    mockApi.createBrowserLoginFlow.mockResolvedValue(emptyFlow);

    const { result } = renderHook(() =>
      useOryFlow("login", undefined, { returnTo: "/home", aal: "aal2" }),
    );

    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/",
        href: "/home",
        replace: true,
      }),
    );
    expect(result.current).toBeNull();
  });

  it("falls back to a first-factor login when an aal2 step-up has no session", async () => {
    // A stale `?aal=aal2` link (or a session that expired since the guard's
    // whoami): Kratos answers `session_aal1_required` — there is nothing to step
    // up, so this must become an ordinary sign-in, not a dead end.
    const firstFactor = {
      id: "plain-flow",
      requested_aal: "aal1",
      ui: { nodes: [] },
    };
    mockApi.createBrowserLoginFlow
      .mockRejectedValueOnce({
        response: new Response(
          JSON.stringify({ error: { id: "session_aal1_required" } }),
          { status: 401 },
        ),
      })
      .mockResolvedValueOnce(firstFactor);

    const { result } = renderHook(() =>
      useOryFlow("login", undefined, { aal: "aal2" }),
    );

    await waitFor(() => expect(result.current).toBe(firstFactor));
    expect(mockApi.createBrowserLoginFlow).toHaveBeenLastCalledWith(
      expect.not.objectContaining({ aal: "aal2" }),
    );
  });

  it("navigates on — without probing for a step-up — when a session is already available", async () => {
    // The altitude fix (w4/m17): the hook no longer mints a trial aal2 flow to
    // find out whether a second factor is owed. The session fetch already knows;
    // absent an `aal` param there is nothing left to prove, so just move on.
    mockApi.createBrowserLoginFlow.mockRejectedValue(sessionAlreadyAvailable());

    renderHook(() => useOryFlow("login", undefined, { returnTo: "/x" }));

    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/",
        href: "/x",
        replace: true,
      }),
    );
    expect(mockApi.createBrowserLoginFlow).toHaveBeenCalledTimes(1);
  });

  it("does not attempt an aal2 step-up for non-login flows", async () => {
    // settings/registration/recovery never step up — a session_already_available
    // there is not a second-factor situation.
    mockApi.createBrowserSettingsFlow.mockRejectedValue(
      sessionAlreadyAvailable(),
    );

    renderHook(() => useOryFlow("settings", undefined, { returnTo: "/y" }));

    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/",
        href: "/y",
        replace: true,
      }),
    );
    // only the single settings-flow attempt; no aal2 login retry.
    expect(mockApi.createBrowserLoginFlow).not.toHaveBeenCalled();
  });
});
