import { describe, expect, it } from "vitest";
import type { ParsedLocation } from "@tanstack/react-router";
import {
  RENDER_NEW_LANDING,
  envGroupAliasPath,
  newPathLanding,
  organizationAliasPath,
  redirectPreservingSuffix,
  redirectRenderAlias,
  renderCreateSlugLanding,
  resourceAliasPath,
  sandboxGroupAliasPath,
} from "../render-alias";

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

  // Render's cron Builds and Runs pages (changelog 2026-09-03) land on bex's
  // (w2/m360).
  it("lands a cron job's Builds and Runs deep links on bex's pages", () => {
    for (const page of ["builds", "runs"]) {
      expect(
        hrefOf("cron", `srv-abc/${page}`, locationOf(`/cron/srv-abc/${page}`)),
      ).toBe(`/services/srv-abc/${page}`);
    }
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
  // generic join onto the nonexistent `/databases/new`. The create pages now
  // live at Render's own paths, so each segment's /new is its own page.
  it("lands each segment's /new on Render's own create path", () => {
    const expected: Record<string, string> = {
      web: "/web/new",
      worker: "/worker/new",
      pserv: "/pserv/new",
      static: "/static/new",
      cron: "/cron/new",
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
    expect(hrefOf("d", "new", locationOf("/d/new"))).toBe("/new/database");
    expect(hrefOf("r", "new", locationOf("/r/new"))).toBe("/new/redis");
  });

  it("carries an incoming query onto a bare create landing", () => {
    expect(hrefOf("d", "new", locationOf("/d/new", "?from=cli"))).toBe(
      "/new/database?from=cli",
    );
  });

  it("keeps a create deep link's contextual query", () => {
    expect(
      hrefOf("cron", "new", locationOf("/cron/new", "?projectId=prj-1")),
    ).toBe("/cron/new?projectId=prj-1");
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

/** A 24-character id of a prefix, as bex and Render mint them. */
function idOf(prefix: string): string {
  return `${prefix}-${"d".repeat(20)}`;
}

function suffixedHref(path: string, location: ParsedLocation) {
  try {
    redirectPreservingSuffix(path, location);
  } catch (thrown) {
    return (thrown as { options?: { href?: string } }).options?.href;
  }
  return undefined;
}

describe("redirectPreservingSuffix", () => {
  it("keeps a landing's own section and puts the incoming query before it", () => {
    expect(
      suffixedHref(
        "/workspace/settings#build-pipeline",
        locationOf("/settings/spend-limit", "?from=cli#ignored"),
      ),
    ).toBe("/workspace/settings?from=cli#build-pipeline");
  });

  it("folds the incoming query into a landing's own and keeps the hash", () => {
    expect(
      suffixedHref(
        "/env-groups?new=env-group",
        locationOf("/new/env-group", "?from=cli#top"),
      ),
    ).toBe("/env-groups?new=env-group&from=cli#top");
  });
});

// Render's `/:serviceTypeSlug/new` loader (`Ez` over `Tz`, bundle read
// 2026-11-27, w3/m705).
describe("renderCreateSlugLanding", () => {
  it.each([
    ["web", "/web/new"],
    ["WEB", "/web/new"],
    ["Worker", "/worker/new"],
    ["pserv", "/pserv/new"],
    ["static", "/static/new"],
    ["cron", "/cron/new"],
    ["background-worker", "/worker/new"],
    ["background-workers", "/worker/new"],
    ["blueprint", "/blueprint/new"],
    ["Blueprints", "/blueprint/new"],
    ["database", "/new/database"],
    ["postgres", "/new/database"],
    ["POSTGRESQL", "/new/database"],
    ["env-group", "/new/env-group"],
    ["env-groups", "/new/env-group"],
    ["kv", "/new/redis"],
    ["key-value", "/new/redis"],
    ["redis", "/new/redis"],
  ])("opens %s's create page", (slug, landing) => {
    expect(renderCreateSlugLanding(slug)).toBe(landing);
  });

  it.each(["nope", "workflow", "team", "constructor", "__proto__", ""])(
    "sends %j to the /new landing",
    (slug) => {
      expect(renderCreateSlugLanding(slug)).toBe(RENDER_NEW_LANDING);
    },
  );
});

describe("newPathLanding", () => {
  it("opens Render's /new/team as bex's New Workspace", () => {
    expect(newPathLanding("team")).toBe("/new/workspace");
    expect(newPathLanding("TEAM")).toBe("/new/workspace");
  });

  it("reads bex's own /new pages and the create slugs case-insensitively", () => {
    expect(newPathLanding("Project")).toBe("/new/project");
    expect(newPathLanding("WORKSPACE")).toBe("/new/workspace");
    expect(newPathLanding("PostgreSQL")).toBe("/new/database");
    expect(newPathLanding("WEB")).toBe("/web/new");
  });

  it("sends /new and an unknown slug to the /new landing", () => {
    expect(newPathLanding(undefined)).toBe(RENDER_NEW_LANDING);
    expect(newPathLanding("nope")).toBe(RENDER_NEW_LANDING);
  });
});

describe("envGroupAliasPath", () => {
  it("opens the group, whatever the sub-path", () => {
    expect(envGroupAliasPath(["evg-x"])).toBe("/env-groups/evg-x");
    expect(envGroupAliasPath(["evg-x", "settings", "more"])).toBe(
      "/env-groups/evg-x",
    );
  });

  it("opens New Environment Group for Render's create slug", () => {
    expect(envGroupAliasPath(["new"])).toBe("/new/env-group");
  });

  it("names nothing without an id", () => {
    expect(envGroupAliasPath([])).toBeUndefined();
  });
});

describe("sandboxGroupAliasPath", () => {
  const base = "/sandbox-groups/sbg-x";
  it.each([
    [["sbg-x"], base],
    [["sbg-x", "sandboxes"], base],
    [["sbg-x", "sandboxes", "sbx-y"], `${base}?sandbox=sbx-y`],
    [["sbg-x", "sandboxes", "sbx-y", "events"], `${base}?sandbox=sbx-y`],
    [
      ["sbg-x", "sandboxes", "sbx-y", "configurations"],
      `${base}?sandbox=sbx-y&tab=details`,
    ],
    [
      ["sbg-x", "sandboxes", "sbx-y", "snapshots"],
      `${base}?sandbox=sbx-y&tab=snapshots`,
    ],
    [["sbg-x", "sandboxes", "sbx-y", "shell"], `${base}?sandbox=sbx-y`],
    [["sbg-x", "snapshots"], `${base}/snapshots`],
    [["sbg-x", "snapshots", "snp-z"], `${base}/snapshots?snapshot=snp-z`],
    [["sbg-x", "builds"], `${base}/images`],
    [
      ["sbg-x", "builds", "agent runtime"],
      `${base}/images?image=agent+runtime`,
    ],
    [["sbg-x", "settings"], `${base}/settings`],
    [["sbg-x", "unknown", "page"], base],
    [["new"], "/?new=sandbox-group"],
  ])("maps %j to %s", (parts, landing) => {
    expect(sandboxGroupAliasPath(parts)).toBe(landing);
  });

  it("names nothing without an id", () => {
    expect(sandboxGroupAliasPath([])).toBeUndefined();
  });
});

describe("organizationAliasPath", () => {
  it.each([
    [["org-x"], "/org/org-x/workspaces"],
    [["org-x", "members"], "/org/org-x/members"],
    [["org-x", "settings"], "/org/org-x/settings"],
    [["org-x", "new", "workspace"], "/org/org-x/workspaces?new=workspace"],
    [["org-x", "anything", "else"], "/org/org-x/workspaces"],
  ])("maps %j to %s", (parts, landing) => {
    expect(organizationAliasPath(parts)).toBe(landing);
  });

  it("names nothing without an id", () => {
    expect(organizationAliasPath([])).toBeUndefined();
  });
});

// Render's ResourceRedirector (bundle read 2026-11-27, w3/m705).
describe("resourceAliasPath", () => {
  it.each([
    ["srv", "/services"],
    ["crn", "/services"],
    ["dpg", "/databases"],
    ["red", "/keyvalue"],
    ["exs", "/blueprints"],
    ["blp", "/blueprints"],
    ["whk", "/webhook"],
  ])("opens a %s- id's page with its sub-path", (prefix, base) => {
    const id = idOf(prefix);
    expect(resourceAliasPath([id])).toBe(`${base}/${id}`);
    expect(resourceAliasPath([id, "settings"])).toBe(`${base}/${id}/settings`);
  });

  it("opens an environment group, a sandbox group, a sandbox and a snapshot", () => {
    const evg = idOf("evg");
    const sbg = idOf("sbg");
    expect(resourceAliasPath([evg, "anything"])).toBe(`/env-groups/${evg}`);
    expect(resourceAliasPath([sbg, "settings"])).toBe(
      `/sandbox-groups/${sbg}/settings`,
    );
    expect(resourceAliasPath([idOf("sbx"), "events"])).toBe(
      `/sandboxes/${idOf("sbx")}`,
    );
    expect(resourceAliasPath([idOf("snp")])).toBe(
      `/sandbox-snapshots/${idOf("snp")}`,
    );
  });

  it("sends a project's id home, as Render's redirector does", () => {
    expect(resourceAliasPath([idOf("prj")])).toBe("/");
  });

  it("sends an unknown prefix, a short id and no id home", () => {
    expect(resourceAliasPath([idOf("dep")])).toBe("/");
    expect(resourceAliasPath([idOf("SRV")])).toBe("/");
    expect(resourceAliasPath(["srv-short"])).toBe("/");
    expect(resourceAliasPath(["nonsense"])).toBe("/");
    expect(resourceAliasPath([])).toBe("/");
  });

  it("finds nothing for a workflow id, which bex never mints", () => {
    for (const prefix of ["wfl", "tsk", "trn", "flw", "frn"]) {
      expect(resourceAliasPath([idOf(prefix)])).toBeUndefined();
    }
  });
});
