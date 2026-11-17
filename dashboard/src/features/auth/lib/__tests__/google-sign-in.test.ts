import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { KRATOS_PUBLIC_URL } from "@/common/lib/ory/config";
import { startGoogleSignIn } from "../google-sign-in";

const RETURN_TO = "https://dashboard.bex.co/services/srv-1?tab=logs";
const GOOGLE_AUTHORIZE =
  "https://accounts.google.com/o/oauth2/v2/auth?client_id=bex&state=s";
const REAUTHENTICATE = `${KRATOS_PUBLIC_URL}/self-service/login/browser?refresh=true&return_to=x`;

function csrfNode(value: string) {
  return {
    type: "input",
    group: "default",
    attributes: {
      name: "csrf_token",
      type: "hidden",
      value,
      node_type: "input",
    },
    messages: [],
    meta: {},
  };
}

/** The oidc button Kratos's settings flow shows for Google: "link" while the
 *  identity has no Google credential, "unlink" once it has one. */
function googleNode(name: "link" | "unlink") {
  return {
    type: "input",
    group: "oidc",
    attributes: { name, type: "submit", value: "google", node_type: "input" },
    messages: [],
    meta: {},
  };
}

function flow(kind: "login" | "settings", id: string, nodes: unknown[]) {
  return {
    id,
    type: "browser",
    expires_at: "2030-01-01T00:00:00Z",
    issued_at: "2026-11-17T00:00:00Z",
    request_url: `${KRATOS_PUBLIC_URL}/self-service/${kind}/browser`,
    state: kind === "settings" ? "show_form" : "choose_method",
    ...(kind === "login" ? { refresh: true } : {}),
    ...(kind === "settings"
      ? { identity: { id: "identity-1", schema_id: "default", traits: {} } }
      : {}),
    ui: {
      action: `${KRATOS_PUBLIC_URL}/self-service/${kind}?flow=${id}`,
      method: "POST",
      nodes,
    },
  };
}

const UNLINKED = () =>
  flow("settings", "settings-1", [csrfNode("csrf-set"), googleNode("link")]);
const LINKED = () =>
  flow("settings", "settings-1", [csrfNode("csrf-set"), googleNode("unlink")]);
const LOGIN = () => flow("login", "login-1", [csrfNode("csrf-login")]);

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

const toGoogle = () =>
  json(
    {
      error: { id: "browser_location_change_required", code: 422 },
      redirect_browser_to: GOOGLE_AUTHORIZE,
    },
    422,
  );

type Call = { url: URL; init?: RequestInit };

/** Kratos's public API: both flows' creation and submits. */
function mockKratos({
  settings = () => json(UNLINKED()),
  settingsSubmit = toGoogle,
  login = () => json(LOGIN()),
  loginSubmit = toGoogle,
}: {
  settings?: (url: URL) => Response;
  settingsSubmit?: () => Response;
  login?: (url: URL) => Response;
  loginSubmit?: () => Response;
} = {}) {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(String(input instanceof Request ? input.url : input));
      calls.push({ url, init });
      switch (url.pathname) {
        case "/self-service/settings/browser":
          return settings(url);
        case "/self-service/settings":
          return settingsSubmit();
        case "/self-service/login/browser":
          return login(url);
        case "/self-service/login":
          return loginSubmit();
      }
      return json({}, 404);
    }),
  );
  const at = (path: string) => calls.filter((c) => c.url.pathname === path);
  return {
    calls,
    settingsCreates: () => at("/self-service/settings/browser"),
    settingsSubmits: () => at("/self-service/settings"),
    loginCreates: () => at("/self-service/login/browser"),
    loginSubmits: () => at("/self-service/login"),
  };
}

const assign = vi.fn();

beforeEach(() => {
  assign.mockReset();
  vi.stubGlobal("location", { assign, href: RETURN_TO });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("startGoogleSignIn (w7/m465)", () => {
  // Kratos v26.2.0: a refresh login with Google leaves an identity with no
  // Google credential signed in as before; linking through the settings flow
  // adds the Google sign-in to the session at once.
  it("links Google through the settings flow when the identity has none, and follows Kratos to Google", async () => {
    const kratos = mockKratos();

    await expect(startGoogleSignIn(RETURN_TO)).resolves.toBe(true);

    const create = kratos.settingsCreates()[0].url;
    expect(create.searchParams.get("return_to")).toBe(RETURN_TO);
    const [submit] = kratos.settingsSubmits();
    expect(submit.url.searchParams.get("flow")).toBe("settings-1");
    expect(submit.init?.method).toBe("POST");
    // The body Ory Elements' own "Link" button sends: the SDK's oidc
    // settings model carries no csrf_token, and Kratos checks none there.
    expect(JSON.parse(String(submit.init?.body))).toEqual({
      method: "oidc",
      link: "google",
    });
    // The session and CSRF cookies ride along: Kratos is another host.
    expect(submit.init?.credentials).toBe("include");
    // No login flow at all on this path.
    expect(kratos.loginCreates()).toHaveLength(0);

    expect(assign).toHaveBeenCalledWith(GOOGLE_AUTHORIZE);
  });

  it("re-authenticates with Google through a refresh login when Google is already linked", async () => {
    const kratos = mockKratos({ settings: () => json(LINKED()) });

    await expect(startGoogleSignIn(RETURN_TO)).resolves.toBe(true);

    // Nothing is submitted to the settings flow: "unlink" is not touched.
    expect(kratos.settingsSubmits()).toHaveLength(0);
    const create = kratos.loginCreates()[0].url;
    expect(create.searchParams.get("refresh")).toBe("true");
    expect(create.searchParams.get("return_to")).toBe(RETURN_TO);
    const [submit] = kratos.loginSubmits();
    expect(submit.url.searchParams.get("flow")).toBe("login-1");
    expect(JSON.parse(String(submit.init?.body))).toEqual({
      method: "oidc",
      provider: "google",
      csrf_token: "csrf-login",
    });
    expect(assign).toHaveBeenCalledWith(GOOGLE_AUTHORIZE);
  });

  it("takes the refresh login path when the settings flow offers no Google link at all", async () => {
    const kratos = mockKratos({
      settings: () => json(flow("settings", "settings-1", [csrfNode("c")])),
    });

    await startGoogleSignIn(RETURN_TO);

    expect(kratos.settingsSubmits()).toHaveLength(0);
    expect(kratos.loginSubmits()).toHaveLength(1);
  });

  // A session too old to be privileged may not change settings: Kratos
  // answers 403 and names where to re-authenticate. Going there is right.
  it("follows Kratos's re-authentication when the link needs a privileged session", async () => {
    mockKratos({
      settingsSubmit: () =>
        json(
          {
            error: { id: "session_refresh_required", code: 403 },
            redirect_browser_to: REAUTHENTICATE,
          },
          403,
        ),
    });

    await expect(startGoogleSignIn(RETURN_TO)).resolves.toBe(true);
    expect(assign).toHaveBeenCalledWith(REAUTHENTICATE);
  });

  it("drops a return_to Kratos does not allow rather than failing", async () => {
    const forbidden = () =>
      json(
        { error: { id: "self_service_flow_return_to_forbidden", code: 400 } },
        400,
      );
    const kratos = mockKratos({
      settings: (url) =>
        url.searchParams.has("return_to") ? forbidden() : json(LINKED()),
      login: (url) =>
        url.searchParams.has("return_to") ? forbidden() : json(LOGIN()),
    });

    await startGoogleSignIn(RETURN_TO);

    const settings = kratos.settingsCreates();
    expect(settings).toHaveLength(2);
    expect(settings[1].url.searchParams.has("return_to")).toBe(false);
    const logins = kratos.loginCreates();
    expect(logins).toHaveLength(2);
    expect(logins[1].url.searchParams.get("refresh")).toBe("true");
    expect(logins[1].url.searchParams.has("return_to")).toBe(false);
    expect(assign).toHaveBeenCalledWith(GOOGLE_AUTHORIZE);
  });

  it.each([
    ["the link", { settingsSubmit: () => json(UNLINKED(), 400) }],
    [
      "the refresh login",
      {
        settings: () => json(LINKED()),
        loginSubmit: () => json(LOGIN(), 400),
      },
    ],
  ])(
    "rejects, and goes nowhere, when Kratos answers %s with no destination",
    async (_, opts) => {
      // Google not configured on this installation: Kratos answers the submit
      // with the flow again and a message, not a redirect.
      mockKratos(opts);

      await expect(startGoogleSignIn(RETURN_TO)).rejects.toBeDefined();
      expect(assign).not.toHaveBeenCalled();
    },
  );

  it("rejects when no flow can be minted", async () => {
    mockKratos({
      settings: () => json({ error: { id: "security_csrf_violation" } }, 403),
    });

    await expect(startGoogleSignIn(RETURN_TO)).rejects.toBeDefined();
    expect(assign).not.toHaveBeenCalled();
  });
});
