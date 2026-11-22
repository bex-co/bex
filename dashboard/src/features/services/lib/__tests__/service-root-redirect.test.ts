import { describe, expect, it } from "vitest";
import { isRedirect } from "@tanstack/react-router";
import {
  redirectBuildsByType,
  redirectCronCompute,
  redirectRunsByType,
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

/** The redirect a beforeLoad helper throws, or null when it lets the page render. */
async function thrown(load: Promise<unknown>): Promise<unknown> {
  try {
    await load;
    return null;
  } catch (error) {
    return error;
  }
}

describe("a cron job's Runs, Deploys and Compute URLs (w3/m554)", () => {
  it("lands a cron job's root on Runs, the page Render shows at /cron/<id>", () => {
    expect(caught("srv-cron", "cron_job")).toMatchObject({
      options: {
        to: "/services/$serviceId/runs",
        params: { serviceId: "srv-cron" },
        replace: true,
      },
    });
  });

  it("renders Runs for a cron job, or when the type is unknown", async () => {
    expect(
      await thrown(redirectRunsByType(clientFor("cron_job"), "srv-1")),
    ).toBeNull();
    expect(
      await thrown(redirectRunsByType(clientFor(null), "srv-1")),
    ).toBeNull();
  });

  it("sends another type's Runs URL to its primary tab", async () => {
    expect(
      await thrown(redirectRunsByType(clientFor("web_service"), "srv-1")),
    ).toMatchObject({ options: { to: "/services/$serviceId/deploys" } });
    expect(
      await thrown(redirectRunsByType(clientFor("static_site"), "srv-1")),
    ).toMatchObject({ options: { to: "/static/$serviceId/events" } });
  });

  it("lands a cron job's Builds URL on Deploys, keeping its query", async () => {
    for (const type of ["cron_job", null]) {
      const error = await thrown(
        redirectBuildsByType(clientFor(type), "srv-1", undefined),
      );
      expect(isRedirect(error)).toBe(true);
      expect(error).toMatchObject({
        options: {
          to: "/services/$serviceId/deploys",
          params: { serviceId: "srv-1" },
          search: true,
          replace: true,
        },
      });
    }
  });

  it("lands a cron job's build on that deploy's detail", async () => {
    expect(
      await thrown(
        redirectBuildsByType(clientFor("cron_job"), "srv-1", "dep-2"),
      ),
    ).toMatchObject({
      options: {
        to: "/services/$serviceId/deploys/$deployId",
        params: { serviceId: "srv-1", deployId: "dep-2" },
        search: true,
        replace: true,
      },
    });
  });

  it("sends another type's Builds URL to its primary tab", async () => {
    expect(
      await thrown(
        redirectBuildsByType(
          clientFor("background_worker"),
          "srv-1",
          undefined,
        ),
      ),
    ).toMatchObject({ options: { to: "/services/$serviceId/deploys" } });
    expect(
      await thrown(
        redirectBuildsByType(clientFor("static_site"), "srv-1", "dep-2"),
      ),
    ).toMatchObject({ options: { to: "/static/$serviceId/events" } });
  });

  it("sends a cron job's Compute URL to Settings' Compute section", async () => {
    const error = await thrown(
      redirectCronCompute(clientFor("cron_job"), "srv-1"),
    );
    expect(isRedirect(error)).toBe(true);
    expect(error).toMatchObject({
      options: {
        to: "/services/$serviceId/settings",
        params: { serviceId: "srv-1" },
        hash: "compute",
        replace: true,
      },
    });
    const search = (
      error as { options: { search: (prev: unknown) => unknown } }
    ).options.search;
    expect(search({ lang: "de" })).toEqual({ lang: "de" });
  });

  it("renders the Compute page for every other type, and an unknown one", async () => {
    for (const type of [
      "web_service",
      "private_service",
      "background_worker",
      null,
    ]) {
      expect(
        await thrown(redirectCronCompute(clientFor(type), "srv-1")),
      ).toBeNull();
    }
  });
});
