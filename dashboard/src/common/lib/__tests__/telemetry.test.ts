import { afterEach, describe, expect, it, vi } from "vitest";

import {
  contentGroup,
  pageLocation,
  pageReferrer,
  telemetryLabel,
  trackEvent,
  trackPageView,
} from "../telemetry";

afterEach(() => {
  delete window.gtag;
});

describe("trackEvent", () => {
  it("forwards to gtag when GA is loaded", () => {
    const gtag = vi.fn();
    window.gtag = gtag;
    trackEvent("ssh_gate_shown", { surface: "shell" });
    expect(gtag).toHaveBeenCalledWith("event", "ssh_gate_shown", {
      surface: "shell",
    });
  });

  it("is a no-op without GA", () => {
    expect(() => trackEvent("ssh_gate_cta_clicked")).not.toThrow();
  });
});

describe("telemetryLabel", () => {
  it("passes identifier-shaped values", () => {
    expect(telemetryLabel("web_service")).toBe("web_service");
    expect(telemetryLabel("claude-fable-5-2")).toBe("claude-fable-5-2");
    expect(telemetryLabel("BAD_USER_INPUT")).toBe("BAD_USER_INPUT");
  });

  it("drops free text, addresses, URLs and non-strings", () => {
    expect(telemetryLabel("my cool service")).toBeUndefined();
    expect(telemetryLabel("ada@example.com")).toBeUndefined();
    expect(telemetryLabel("https://github.com/acme/app")).toBeUndefined();
    expect(telemetryLabel("x".repeat(41))).toBeUndefined();
    expect(telemetryLabel("")).toBeUndefined();
    expect(telemetryLabel(42)).toBeUndefined();
    expect(telemetryLabel(undefined)).toBeUndefined();
  });
});

describe("pageLocation", () => {
  const origin = "https://dashboard.bex.co";

  it("is the route template, never the visited path's query", () => {
    expect(
      pageLocation(
        origin,
        "/auth/verification",
        "?flow=4f1c&code=123456&next=%2Fservices%2Fsrv-abc",
      ),
    ).toBe("https://dashboard.bex.co/auth/verification");
    expect(pageLocation(origin, "/invite", "?token=secret")).toBe(
      "https://dashboard.bex.co/invite",
    );
  });

  it("keeps the campaign parameters GA attributes a session from", () => {
    expect(
      pageLocation(
        origin,
        "/auth/sign-up",
        "?utm_source=hn&utm_medium=social&utm_campaign=launch&gclid=abc&next=%2F",
      ),
    ).toBe(
      "https://dashboard.bex.co/auth/sign-up?utm_source=hn&utm_medium=social&utm_campaign=launch&gclid=abc",
    );
  });
});

describe("pageReferrer", () => {
  const origin = "https://dashboard.bex.co";

  it("keeps another site's origin and path, without its query", () => {
    expect(pageReferrer("https://bex.co/pricing?ref=nav#plans", origin)).toBe(
      "https://bex.co/pricing",
    );
    expect(pageReferrer("https://github.com/", origin)).toBe(
      "https://github.com/",
    );
  });

  it("reduces a same-origin referrer to the origin", () => {
    expect(
      pageReferrer(
        "https://dashboard.bex.co/auth/verification?code=123456",
        origin,
      ),
    ).toBe("https://dashboard.bex.co/");
  });

  it("is empty for no referrer or an unparseable one", () => {
    expect(pageReferrer("", origin)).toBe("");
    expect(pageReferrer("not a url", origin)).toBe("");
  });
});

describe("contentGroup", () => {
  it("is the template's first segment", () => {
    expect(contentGroup("/services/$serviceId/logs")).toBe("services");
    expect(contentGroup("/auth/login")).toBe("auth");
    expect(contentGroup("/")).toBe("home");
  });
});

describe("trackPageView", () => {
  it("sets the scrubbed page context, then sends the page view", () => {
    const gtag = vi.fn();
    window.gtag = gtag;
    const location =
      "https://dashboard.bex.co/services/$serviceId/logs?utm_source=docs";
    trackPageView({
      location,
      template: "/services/$serviceId/logs",
      referrer: "https://bex.co/docs",
    });
    expect(gtag.mock.calls).toEqual([
      [
        "set",
        { page_location: location, page_title: "/services/$serviceId/logs" },
      ],
      [
        "event",
        "page_view",
        { page_referrer: "https://bex.co/docs", content_group: "services" },
      ],
    ]);
  });

  it("is a no-op without GA", () => {
    expect(() =>
      trackPageView({ location: "https://x/", template: "/", referrer: "" }),
    ).not.toThrow();
  });
});
