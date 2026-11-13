import { describe, expect, it } from "vitest";
import { isRedirect } from "@tanstack/react-router";
import {
  redirectHistoryTabByType,
  redirectServiceRoot,
} from "../service-root-redirect";
import type { RouterContext } from "@/common/types/router-context";

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

/** A router client whose ServerDocument read answers `type`. */
function clientFor(type: string | null): RouterContext["client"] {
  return {
    query: async () => ({ data: type === null ? null : { server: { type } } }),
  } as unknown as RouterContext["client"];
}

async function historyRedirect(
  type: string | null,
  tab: "deploys" | "builds" | "runs",
): Promise<unknown> {
  try {
    await redirectHistoryTabByType(clientFor(type), "srv-1", tab);
    return null;
  } catch (error) {
    return error;
  }
}

describe("a cron job's Builds and Runs (w2/m360)", () => {
  it("lands a cron job's root on Builds", () => {
    expect(caught("srv-cron", "cron_job")).toMatchObject({
      options: { to: "/services/$serviceId/builds" },
    });
  });

  it("sends a cron job's Deploys URL to Builds, keeping its filters", async () => {
    const error = await historyRedirect("cron_job", "deploys");
    expect(isRedirect(error)).toBe(true);
    expect(error).toMatchObject({
      options: {
        to: "/services/$serviceId/builds",
        params: { serviceId: "srv-1" },
        replace: true,
      },
    });
    const search = (
      error as { options: { search: (prev: unknown) => unknown } }
    ).options.search;
    expect(search({ status: "failed" })).toEqual({ status: "failed" });
  });

  it("renders Builds and Runs for a cron job, and Deploys for every other type", async () => {
    expect(await historyRedirect("cron_job", "builds")).toBeNull();
    expect(await historyRedirect("cron_job", "runs")).toBeNull();
    expect(await historyRedirect("web_service", "deploys")).toBeNull();
    // Unknown type: render what was asked for.
    expect(await historyRedirect(null, "runs")).toBeNull();
  });

  it("sends another type's Builds or Runs URL to its primary tab", async () => {
    expect(await historyRedirect("web_service", "runs")).toMatchObject({
      options: { to: "/services/$serviceId/deploys" },
    });
    expect(await historyRedirect("background_worker", "builds")).toMatchObject({
      options: { to: "/services/$serviceId/deploys" },
    });
    expect(await historyRedirect("static_site", "runs")).toMatchObject({
      options: { to: "/static/$serviceId/events" },
    });
  });
});
