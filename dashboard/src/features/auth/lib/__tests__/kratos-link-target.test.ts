import { describe, it, expect } from "vitest";
import { kratosLinkTarget } from "@/features/auth/lib/kratos-link-target";

const APP = "https://dashboard.bex.co";
const KRATOS = "https://auth.bex.co";

/** The href Ory Elements actually renders (`initFlowUrl`). */
function oryHref(
  flow: string,
  params: Record<string, string> = {},
  base = KRATOS,
) {
  const url = new URL(`${base}/self-service/${flow}/browser`);
  for (const [k, v] of Object.entries(params)) url.searchParams.set(k, v);
  return url.toString();
}

describe("kratosLinkTarget", () => {
  it("maps every self-service flow onto the route that serves it", () => {
    expect(kratosLinkTarget(oryHref("login"), APP, KRATOS)?.to).toBe(
      "/auth/login",
    );
    expect(kratosLinkTarget(oryHref("registration"), APP, KRATOS)?.to).toBe(
      "/auth/sign-up",
    );
    expect(kratosLinkTarget(oryHref("recovery"), APP, KRATOS)?.to).toBe(
      "/auth/forgot-password",
    );
    expect(kratosLinkTarget(oryHref("verification"), APP, KRATOS)?.to).toBe(
      "/auth/verification",
    );
  });

  it("collapses a root return_to instead of emitting ?next=%2F", () => {
    const target = kratosLinkTarget(
      oryHref("registration", { return_to: `${APP}/` }),
      APP,
      KRATOS,
    );
    expect(target).toEqual({
      to: "/auth/sign-up",
      search: { next: undefined, flow: undefined, login_challenge: undefined },
    });
  });

  // The OAuth2 regression guard: dropping this silently abandons the Hydra
  // authorization the user is in the middle of.
  it("forwards login_challenge to the routes that model it", () => {
    const challenge = "hydra-abc";
    for (const [flow, to] of [
      ["login", "/auth/login"],
      ["registration", "/auth/sign-up"],
    ] as const) {
      const target = kratosLinkTarget(
        oryHref(flow, { login_challenge: challenge }),
        APP,
        KRATOS,
      );
      expect(target?.to).toBe(to);
      expect(
        (target?.search as { login_challenge?: string }).login_challenge,
      ).toBe(challenge);
    }
  });

  it("does not carry login_challenge onto a route without it", () => {
    const target = kratosLinkTarget(
      oryHref("recovery", { login_challenge: "hydra-abc" }),
      APP,
      KRATOS,
    );
    expect(target).toEqual({
      to: "/auth/forgot-password",
      search: { flow: undefined },
    });
  });

  it("preserves a deep return_to including query and hash", () => {
    const target = kratosLinkTarget(
      oryHref("login", { return_to: `${APP}/w/tea-1/settings?tab=x#y` }),
      APP,
      KRATOS,
    );
    expect((target?.search as { next?: string }).next).toBe(
      "/w/tea-1/settings?tab=x#y",
    );
  });

  it.each([
    "https://evil.com/",
    "//evil.com",
    "/\\evil.com",
    "https:%2f%2fevil.com",
  ])("refuses to route a foreign return_to (%s)", (returnTo) => {
    const target = kratosLinkTarget(
      oryHref("login", { return_to: returnTo }),
      APP,
      KRATOS,
    );
    expect((target?.search as { next?: string }).next).toBeUndefined();
    // Still routed — only the hostile return_to is dropped.
    expect(target?.to).toBe("/auth/login");
  });

  it("never carries a flow id or synthesizes an aal step-up", () => {
    const target = kratosLinkTarget(
      oryHref("login", { flow: "some-uuid", aal: "aal2" }),
      APP,
      KRATOS,
    );
    expect(target).toEqual({
      to: "/auth/login",
      search: {
        next: undefined,
        flow: undefined,
        login_challenge: undefined,
        aal: undefined,
      },
    });
  });

  it("drops identity_schema, which no route models", () => {
    const target = kratosLinkTarget(
      oryHref("registration", { identity_schema: "default" }),
      APP,
      KRATOS,
    );
    expect(target?.search).not.toHaveProperty("identity_schema");
  });

  it.each([
    `${KRATOS}/self-service/logout?token=t`,
    `${KRATOS}/self-service/settings/browser`,
    `${KRATOS}/self-service/errors?id=1`,
    `${KRATOS}/.well-known/ory/webauthn.js`,
    `${KRATOS}/`,
    "https://evil.com/self-service/login/browser",
    `${APP}/auth/login`,
    "not a url",
  ])("leaves %s alone", (href) => {
    expect(kratosLinkTarget(href, APP, KRATOS)).toBeNull();
  });

  // The documented local-dev proxy mounts Kratos on the app's OWN origin under
  // /kratos, so an origin-only match would swallow every in-app link.
  describe("when Kratos is proxied under a same-origin path prefix", () => {
    const DEV = "http://localhost:5173";
    const PROXY = "http://localhost:5173/kratos";

    it("matches links under the prefix", () => {
      expect(
        kratosLinkTarget(oryHref("login", {}, PROXY), DEV, PROXY)?.to,
      ).toBe("/auth/login");
    });

    it("does not match an app path that merely looks like one", () => {
      expect(
        kratosLinkTarget(`${DEV}/self-service/login/browser`, DEV, PROXY),
      ).toBeNull();
    });
  });
});
