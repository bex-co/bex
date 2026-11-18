import type { Session } from "@ory/client-fetch";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { memoryStorage } from "@/test/mocks/storage";
import {
  SESSION_KEY,
  installRouterTelemetry,
  pageTemplate,
  rememberCheckout,
  sessionEvent,
} from "../telemetry-router";

describe("pageTemplate", () => {
  it("is the deepest match's route template", () => {
    expect(
      pageTemplate([
        { fullPath: "/" },
        { fullPath: "/services/$serviceId" },
        { fullPath: "/services/$serviceId/logs" },
      ]),
    ).toBe("/services/$serviceId/logs");
  });

  it("drops an index route's trailing slash, and keeps the root", () => {
    expect(pageTemplate([{ fullPath: "/services/$serviceId/" }])).toBe(
      "/services/$serviceId",
    );
    expect(pageTemplate([{ fullPath: "/" }])).toBe("/");
    expect(pageTemplate([])).toBe("/");
  });
});

const NOW = Date.parse("2026-11-16T12:00:00Z");

function session(overrides: {
  authenticatedAt: string | Date;
  createdAt: string;
  method?: string;
  provider?: string;
}): Session {
  return {
    id: "ses-1",
    authenticated_at: overrides.authenticatedAt,
    authentication_methods: [
      { method: overrides.method ?? "password", provider: overrides.provider },
    ],
    identity: { id: "idn-1", created_at: overrides.createdAt },
  } as unknown as Session;
}

describe("sessionEvent", () => {
  it("is a sign_up when the identity was created with the session", () => {
    expect(
      sessionEvent(
        session({
          authenticatedAt: "2026-11-16T11:59:30Z",
          createdAt: "2026-11-16T11:59:29Z",
        }),
        NOW,
      ),
    ).toEqual({ event: "sign_up", method: "password" });
  });

  it("is a login for an older identity, named by its OIDC provider", () => {
    expect(
      sessionEvent(
        session({
          authenticatedAt: new Date("2026-11-16T11:58:00Z"),
          createdAt: "2026-10-01T08:00:00Z",
          method: "oidc",
          provider: "github",
        }),
        NOW,
      ),
    ).toEqual({ event: "login", method: "github" });
  });

  // w7/m468: Render's four providers. Kratos names the session's oidc method
  // by the provider id (the `bitbucket` generic provider at Dex's issuer too).
  it.each(["gitlab", "bitbucket", "google"])(
    "names a %s sign-up and login by its provider, not oidc",
    (provider) => {
      expect(
        sessionEvent(
          session({
            authenticatedAt: "2026-11-16T11:59:30Z",
            createdAt: "2026-11-16T11:59:29Z",
            method: "oidc",
            provider,
          }),
          NOW,
        ),
      ).toEqual({ event: "sign_up", method: provider });
      expect(
        sessionEvent(
          session({
            authenticatedAt: "2026-11-16T11:59:30Z",
            createdAt: "2026-10-01T08:00:00Z",
            method: "oidc",
            provider,
          }),
          NOW,
        ),
      ).toEqual({ event: "login", method: provider });
    },
  );

  it("is a login when the identity carries no creation time", () => {
    expect(
      sessionEvent(
        {
          id: "ses-3",
          authenticated_at: "2026-11-16T11:59:30Z",
          authentication_methods: [{ method: "passkey" }],
        } as unknown as Session,
        NOW,
      ),
    ).toEqual({ event: "login", method: "passkey" });
  });

  it("is nothing for a session that authenticated long ago", () => {
    expect(
      sessionEvent(
        session({
          authenticatedAt: "2026-11-15T09:00:00Z",
          createdAt: "2026-11-15T09:00:00Z",
        }),
        NOW,
      ),
    ).toBeNull();
    expect(sessionEvent({ id: "ses-2" } as Session, NOW)).toBeNull();
  });
});

describe("installRouterTelemetry", () => {
  type Rendered = (event: { toLocation: { pathname: string } }) => void;
  let gtag: ReturnType<typeof vi.fn>;
  let matches: Array<{ fullPath: string; context?: unknown }>;
  let rendered: Rendered;

  const install = () => {
    const subscribe = vi.fn((_type: string, fn: Rendered) => {
      rendered = fn;
    });
    installRouterTelemetry({
      subscribe,
      state: {
        get matches() {
          return matches;
        },
      },
    } as unknown as Parameters<typeof installRouterTelemetry>[0]);
    return subscribe;
  };
  const events = () =>
    gtag.mock.calls.filter(([command]) => command === "event");

  beforeEach(() => {
    gtag = vi.fn();
    window.gtag = gtag as unknown as Window["gtag"];
    // The suite's default localStorage is a no-op mock (test/setup.ts).
    vi.stubGlobal("localStorage", memoryStorage());
    window.sessionStorage.clear();
    window.history.replaceState(null, "", "/");
    matches = [{ fullPath: "/" }];
    vi.useFakeTimers({ now: NOW });
  });
  afterEach(() => {
    delete window.gtag;
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("does not subscribe without GA", () => {
    delete window.gtag;
    expect(install()).not.toHaveBeenCalled();
  });

  it("reports one page view per path, by template, chained by referrer", () => {
    install();
    window.history.replaceState(null, "", "/services/srv-abc?tab=x");
    matches = [{ fullPath: "/" }, { fullPath: "/services/$serviceId/" }];
    rendered({ toLocation: { pathname: "/services/srv-abc" } });
    // The same path rendering again (a search-param change, a revalidation).
    rendered({ toLocation: { pathname: "/services/srv-abc" } });
    window.history.replaceState(null, "", "/services/srv-abc/logs");
    matches = [{ fullPath: "/" }, { fullPath: "/services/$serviceId/logs" }];
    rendered({ toLocation: { pathname: "/services/srv-abc/logs" } });

    const origin = window.location.origin;
    expect(gtag.mock.calls.filter(([c]) => c === "set")).toEqual([
      [
        "set",
        {
          page_location: `${origin}/services/$serviceId`,
          page_title: "/services/$serviceId",
        },
      ],
      [
        "set",
        {
          page_location: `${origin}/services/$serviceId/logs`,
          page_title: "/services/$serviceId/logs",
        },
      ],
    ]);
    expect(events()).toEqual([
      ["event", "page_view", { page_referrer: "", content_group: "services" }],
      [
        "event",
        "page_view",
        {
          page_referrer: `${origin}/services/$serviceId`,
          content_group: "services",
        },
      ],
    ]);
    expect(JSON.stringify(gtag.mock.calls)).not.toContain("srv-abc");
  });

  it("reports a fresh session once, across reloads and tabs", () => {
    const fresh = session({
      authenticatedAt: "2026-11-16T11:59:30Z",
      createdAt: "2026-11-16T11:59:29Z",
    });
    matches = [{ fullPath: "/", context: { session: fresh } }];
    install();
    rendered({ toLocation: { pathname: "/" } });
    rendered({ toLocation: { pathname: "/services" } });
    install(); // a reload
    rendered({ toLocation: { pathname: "/" } });

    expect(events().filter(([, name]) => name === "sign_up")).toEqual([
      ["event", "sign_up", { method: "password" }],
    ]);
  });

  it("records a session that was already signed in without reporting it", () => {
    const old = session({
      authenticatedAt: "2026-11-10T09:00:00Z",
      createdAt: "2026-11-10T09:00:00Z",
    });
    matches = [{ fullPath: "/", context: { session: old } }];
    install();
    rendered({ toLocation: { pathname: "/" } });
    expect(events().map(([, name]) => name)).toEqual(["page_view"]);
    expect(window.localStorage.getItem(SESSION_KEY)).toBe("ses-1");
  });

  it("reports Stripe Checkout's return once, only after a checkout began", () => {
    install();
    window.history.replaceState(null, "", "/setup/payment?billing=success");
    matches = [{ fullPath: "/setup/payment" }];
    rendered({ toLocation: { pathname: "/setup/payment" } });
    expect(events().map(([, name]) => name)).toEqual(["page_view"]);

    rememberCheckout();
    install(); // the full-page return from Stripe
    rendered({ toLocation: { pathname: "/setup/payment" } });
    install(); // a reload of the return URL
    rendered({ toLocation: { pathname: "/setup/payment" } });
    expect(
      events().filter(([, name]) => name === "add_payment_info"),
    ).toHaveLength(1);
  });

  it("reports a cancelled checkout", () => {
    rememberCheckout();
    install();
    window.history.replaceState(null, "", "/setup/payment?billing=cancelled");
    rendered({ toLocation: { pathname: "/setup/payment" } });
    expect(events().map(([, name]) => name)).toContain("checkout_cancelled");
  });
});
