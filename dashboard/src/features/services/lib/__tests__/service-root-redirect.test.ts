import { describe, expect, it } from "vitest";
import { isRedirect } from "@tanstack/react-router";
import { redirectServiceRoot } from "../service-root-redirect";

/** Capture the redirect `redirectServiceRoot` throws (it never returns). */
function caught(serviceId: string, type?: string | null): unknown {
  try {
    redirectServiceRoot(serviceId, type);
    throw new Error("expected a redirect");
  } catch (error) {
    return error;
  }
}

describe("redirectServiceRoot", () => {
  it("lands a compute service on Deploys", () => {
    const error = caught("srv-web", "web_service");
    expect(isRedirect(error)).toBe(true);
    expect(error).toMatchObject({
      options: {
        to: "/services/$serviceId/deploys",
        params: { serviceId: "srv-web" },
        replace: true,
      },
    });
  });

  it("lands a static_site on Events under /static (no Deploys tab — Render parity, w5/m57)", () => {
    const error = caught("srv-static", "static_site");
    expect(isRedirect(error)).toBe(true);
    expect(error).toMatchObject({
      options: {
        to: "/static/$serviceId/events",
        params: { serviceId: "srv-static" },
        replace: true,
      },
    });
  });

  it("falls back to Deploys for an unknown/absent type", () => {
    expect(caught("srv-x")).toMatchObject({
      options: { to: "/services/$serviceId/deploys" },
    });
    expect(caught("srv-y", null)).toMatchObject({
      options: { to: "/services/$serviceId/deploys" },
    });
  });
});
