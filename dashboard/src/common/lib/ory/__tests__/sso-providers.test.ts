import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { UiNode } from "@ory/client-fetch";
import { memoryStorage } from "@/test/mocks/storage";
import {
  arrangeSsoProviders,
  loadSsoProviders,
  rememberSsoProviders,
  rememberedSsoProviders,
  resetSsoProvidersCache,
  SSO_PROVIDERS_KEY,
  ssoProviderOf,
  ssoProvidersOfNodes,
  ssoRowColumns,
} from "../sso-providers";

// Render's login and sign-up pages (checked 2026-11-17): GitHub | Google, then
// "More options" for GitLab | Bitbucket. Kratos sorts its nodes by provider id.
describe("arrangeSsoProviders", () => {
  it("puts GitHub and Google first and keeps GitLab and Bitbucket for More options", () => {
    expect(
      arrangeSsoProviders(["bitbucket", "github", "gitlab", "google"]),
    ).toEqual({ primary: ["github", "google"], more: ["gitlab", "bitbucket"] });
  });

  it("shows GitLab and Bitbucket directly when there is nothing to put above them", () => {
    expect(arrangeSsoProviders(["bitbucket", "gitlab"])).toEqual({
      primary: ["gitlab", "bitbucket"],
      more: [],
    });
  });

  it("keeps a lone first-row provider above More options", () => {
    expect(arrangeSsoProviders(["gitlab", "google"])).toEqual({
      primary: ["google"],
      more: ["gitlab"],
    });
  });

  it("lists a provider bex does not know in the first row, after GitHub and Google", () => {
    expect(
      arrangeSsoProviders(["microsoft", "google", "apple", "github"]),
    ).toEqual({
      primary: ["github", "google", "microsoft", "apple"],
      more: [],
    });
  });

  it("arranges any item by the provider it names", () => {
    const items = [{ id: "gitlab" }, { id: "github" }];
    expect(arrangeSsoProviders(items, ({ id }) => id)).toEqual({
      primary: [{ id: "github" }],
      more: [{ id: "gitlab" }],
    });
  });

  it("offers nothing for no providers", () => {
    expect(arrangeSsoProviders([])).toEqual({ primary: [], more: [] });
  });
});

describe("ssoRowColumns", () => {
  it("gives each button of a row its own column, three at most", () => {
    expect([1, 2, 3, 4].map(ssoRowColumns)).toEqual([
      "grid-cols-1",
      "grid-cols-2",
      "grid-cols-3",
      "grid-cols-3",
    ]);
  });
});

function node(group: string, name: string, value: string): UiNode {
  return {
    type: "input",
    group,
    attributes: { name, value, type: "submit", node_type: "input" },
    messages: [],
    meta: {},
  } as unknown as UiNode;
}

/** The nodes Kratos v26.2.0 puts on a login flow for four providers. */
const KRATOS_LOGIN_NODES: UiNode[] = [
  node("default", "csrf_token", "x"),
  ...["bitbucket", "github", "gitlab", "google"].map((provider) =>
    node("oidc", "provider", provider),
  ),
  node("password", "method", "password"),
];

describe("ssoProviderOf", () => {
  it("names the provider of a sign-in, link or unlink button", () => {
    expect(ssoProviderOf(node("oidc", "provider", "gitlab"))).toBe("gitlab");
    expect(ssoProviderOf(node("oidc", "link", "bitbucket"))).toBe("bitbucket");
    expect(ssoProviderOf(node("oidc", "unlink", "github"))).toBe("github");
  });

  it("names nothing for any other node", () => {
    expect(ssoProviderOf(node("password", "method", "password"))).toBe(
      undefined,
    );
    expect(ssoProviderOf(node("oidc", "upstream_parameters", "x"))).toBe(
      undefined,
    );
    expect(
      ssoProviderOf({
        type: "text",
        group: "oidc",
        attributes: { node_type: "text", id: "t", text: {} },
        messages: [],
        meta: {},
      } as unknown as UiNode),
    ).toBe(undefined);
    expect(ssoProviderOf(undefined)).toBe(undefined);
  });
});

describe("ssoProvidersOfNodes", () => {
  it("reads the provider ids, in the flow's order", () => {
    expect(ssoProvidersOfNodes(KRATOS_LOGIN_NODES)).toEqual([
      "bitbucket",
      "github",
      "gitlab",
      "google",
    ]);
  });

  it("is empty for a flow with no providers", () => {
    expect(ssoProvidersOfNodes([KRATOS_LOGIN_NODES[0]!])).toEqual([]);
  });
});

function kratosLoginFlow(nodes: UiNode[]) {
  return {
    id: "flow-1",
    type: "api",
    expires_at: "2026-11-17T13:00:00Z",
    issued_at: "2026-11-17T12:00:00Z",
    request_url: "http://kratos/self-service/login/api",
    state: "choose_method",
    ui: { action: "http://kratos/x", method: "POST", nodes },
  };
}

describe("loadSsoProviders in the browser", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", memoryStorage());
  });

  afterEach(() => {
    resetSsoProvidersCache();
    vi.unstubAllGlobals();
  });

  it("never asks Kratos: it answers with what a flow last offered", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    await expect(loadSsoProviders()).resolves.toBeNull();
    rememberSsoProviders(["github", "gitlab"]);
    await expect(loadSsoProviders()).resolves.toEqual(["github", "gitlab"]);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("keeps the providers across page loads in localStorage", () => {
    rememberSsoProviders(["github", "google", "gitlab", "bitbucket"]);
    expect(JSON.parse(window.localStorage.getItem(SSO_PROVIDERS_KEY)!)).toEqual(
      ["github", "google", "gitlab", "bitbucket"],
    );

    // A new page: nothing in memory, the stored list read back.
    resetSsoProvidersCache();
    expect(rememberedSsoProviders()).toEqual([
      "github",
      "google",
      "gitlab",
      "bitbucket",
    ]);
  });

  it("ignores a stored value that is not a list of provider ids", () => {
    window.localStorage.setItem(SSO_PROVIDERS_KEY, '["github", "<script>"]');
    expect(rememberedSsoProviders()).toBeNull();
    resetSsoProvidersCache();
    window.localStorage.setItem(SSO_PROVIDERS_KEY, "{not json");
    expect(rememberedSsoProviders()).toBeNull();
  });
});

describe("loadSsoProviders on the SSR server", () => {
  let answer: "nodes" | "two" | "down";
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.stubEnv("SSR", true);
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(Date.parse("2026-11-17T12:00:00Z"));
    answer = "down";
    fetchMock = vi.fn(async () => {
      if (answer === "down") throw new TypeError("network down");
      const nodes =
        answer === "two" ? KRATOS_LOGIN_NODES.slice(0, 3) : KRATOS_LOGIN_NODES;
      return new Response(JSON.stringify(kratosLoginFlow(nodes)), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    resetSsoProvidersCache();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
  });

  const advance = (ms: number) => vi.setSystemTime(Date.now() + ms);
  // A background refresh settles on the microtask queue.
  const settle = () => new Promise((resolve) => setTimeout(resolve, 0));

  it("asks Kratos's native login flow, without cookies, until it first answers", async () => {
    await expect(loadSsoProviders()).resolves.toBeNull();
    // Not asked again while Kratos just failed.
    advance(14_000);
    await expect(loadSsoProviders()).resolves.toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);

    answer = "nodes";
    advance(2_000);
    await expect(loadSsoProviders()).resolves.toEqual([
      "bitbucket",
      "github",
      "gitlab",
      "google",
    ]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    const [request, init] = fetchMock.mock.calls[1] as [
      string | Request,
      RequestInit,
    ];
    expect(String(request instanceof Request ? request.url : request)).toMatch(
      /\/self-service\/login\/api$/,
    );
    expect(init.credentials).toBe("omit");
  });

  it("serves its answer for five minutes, then the old one while it refreshes in the background", async () => {
    answer = "nodes";
    await expect(loadSsoProviders()).resolves.toHaveLength(4);

    advance(4 * 60_000);
    await expect(loadSsoProviders()).resolves.toHaveLength(4);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    answer = "two";
    advance(60_000);
    // Expired: the last answer at once, and a refresh behind it.
    await expect(loadSsoProviders()).resolves.toHaveLength(4);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    await settle();
    await expect(loadSsoProviders()).resolves.toEqual(["bitbucket", "github"]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("keeps its last answer when a refresh fails", async () => {
    answer = "nodes";
    await loadSsoProviders();

    answer = "down";
    advance(5 * 60_000);
    await expect(loadSsoProviders()).resolves.toHaveLength(4);
    await settle();
    await expect(loadSsoProviders()).resolves.toHaveLength(4);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
