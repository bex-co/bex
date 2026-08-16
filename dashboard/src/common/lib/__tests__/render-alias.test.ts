import { describe, expect, it } from "vitest";
import type { ParsedLocation } from "@tanstack/react-router";
import { redirectRenderAlias } from "../render-alias";

function locationOf(pathname: string, suffix = ""): ParsedLocation {
  return { pathname, href: `${pathname}${suffix}` } as ParsedLocation;
}

/** redirect() throws; capture the thrown redirect's href. */
function hrefOf(
  segment: Parameters<typeof redirectRenderAlias>[0],
  splat: string | undefined,
  location: ParsedLocation,
): string | undefined {
  try {
    redirectRenderAlias(segment, splat, location);
  } catch (thrown) {
    return (thrown as { options?: { href?: string } }).options?.href;
  }
  return undefined;
}

describe("redirectRenderAlias", () => {
  it("maps every service segment to the canonical service route", () => {
    for (const segment of [
      "web",
      "worker",
      "pserv",
      "static",
      "cron",
    ] as const) {
      expect(
        hrefOf(segment, "srv-abc123", locationOf(`/${segment}/srv-abc123`)),
      ).toBe("/services/srv-abc123");
    }
  });

  it("maps the datastore segments", () => {
    expect(hrefOf("d", "dpg-abc123", locationOf("/d/dpg-abc123"))).toBe(
      "/databases/dpg-abc123",
    );
    expect(hrefOf("r", "red-abc123", locationOf("/r/red-abc123"))).toBe(
      "/keyvalue/red-abc123",
    );
  });

  it("preserves sub-paths (Render deploy deep links)", () => {
    expect(
      hrefOf(
        "web",
        "srv-abc/deploys/dep-xyz",
        locationOf("/web/srv-abc/deploys/dep-xyz"),
      ),
    ).toBe("/services/srv-abc/deploys/dep-xyz");
    expect(
      hrefOf("cron", "srv-abc/settings", locationOf("/cron/srv-abc/settings")),
    ).toBe("/services/srv-abc/settings");
  });

  it("preserves the query string and hash", () => {
    expect(
      hrefOf(
        "web",
        "srv-abc/logs",
        locationOf("/web/srv-abc/logs", "?level=error&range=1d#tail"),
      ),
    ).toBe("/services/srv-abc/logs?level=error&range=1d#tail");
  });

  it("sends a bare segment with no id to the overview", () => {
    expect(hrefOf("web", undefined, locationOf("/web"))).toBe("/");
    expect(hrefOf("d", "", locationOf("/d"))).toBe("/");
  });

  // Render's New menu puts creates under the segment (`/web/new`, `/d/new` —
  // live capture 2026-07-16, w1/m45). `/d/new` used to fall through the
  // generic join onto the nonexistent `/databases/new`. Service segments now
  // carry `?type=` so the wizard preselects the type (w5/m47) — otherwise
  // `/cron/new` dropped the caller onto the Web-Service default.
  it("sends each service segment's /new to a type-preselected wizard", () => {
    const expected: Record<string, string> = {
      web: "/services/new?type=web_service",
      worker: "/services/new?type=background_worker",
      pserv: "/services/new?type=private_service",
      static: "/services/new?type=static_site",
      cron: "/services/new?type=cron_job",
    };
    for (const segment of [
      "web",
      "worker",
      "pserv",
      "static",
      "cron",
    ] as const) {
      expect(hrefOf(segment, "new", locationOf(`/${segment}/new`))).toBe(
        expected[segment],
      );
    }
    expect(hrefOf("d", "new", locationOf("/d/new"))).toBe("/?new=database");
    expect(hrefOf("r", "new", locationOf("/r/new"))).toBe("/keyvalue/new");
  });

  it("folds an incoming query into a landing that already has one", () => {
    expect(hrefOf("d", "new", locationOf("/d/new", "?from=cli"))).toBe(
      "/?new=database&from=cli",
    );
    // A service create deep link with its own contextual query folds onto the
    // `?type=` landing: `/cron/new?projectId=x` → `…?type=cron_job&projectId=x`.
    expect(
      hrefOf("cron", "new", locationOf("/cron/new", "?projectId=prj-1")),
    ).toBe("/services/new?type=cron_job&projectId=prj-1");
  });

  it("replaces history instead of stacking the alias entry", () => {
    try {
      redirectRenderAlias("web", "srv-abc", locationOf("/web/srv-abc"));
    } catch (thrown) {
      expect(
        (thrown as { options?: { replace?: boolean } }).options?.replace,
      ).toBe(true);
    }
  });
});
