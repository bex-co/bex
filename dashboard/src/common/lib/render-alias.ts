import { redirect, type ParsedLocation } from "@tanstack/react-router";

/**
 * Render dashboard URL scheme → bex canonical routes
 * (docs/render-artifacts/dashboard-routes.md). Render's API and its official
 * CLI mint `<dashboard>/{segment}/{id}` deep links (`/web/srv-…`, `/d/dpg-…`,
 * `/r/red-…`); bex serves them as thin redirect aliases onto the canonical
 * `/services` / `/databases` / `/keyvalue` routes. Every service segment
 * accepts any service id — the CLI falls back to `web` for unknown service
 * types, so segment↔type validation would break its links.
 */
const RENDER_ALIAS_TARGETS = {
  web: "/services",
  worker: "/services",
  pserv: "/services",
  static: "/services",
  cron: "/services",
  d: "/databases",
  r: "/keyvalue",
} as const;

export type RenderAliasSegment = keyof typeof RENDER_ALIAS_TARGETS;

/**
 * Where each segment's CREATE URL lands (w1/m45, w5/m47). Render's New menu puts
 * service creates under the type segment (`/web/new` … — live capture,
 * 2026-07-16); the generic `${target}/${rest}` join happens to land those on
 * `/services/new` and `/r/new` on `/keyvalue/new`, but sent `/d/new` to a
 * NONEXISTENT `/databases/new` (bex creates databases via a dialog, reached
 * through the overview's URL-owned `?new=database`). Explicit landings make
 * the create shape a contract instead of an accident. Service segments carry
 * `?type=` so the wizard preselects that type (w5/m47) — otherwise `/cron/new`
 * would drop the caller onto the Web-Service default.
 */
export const RENDER_CREATE_LANDINGS: Record<RenderAliasSegment, string> = {
  web: "/services/new?type=web_service",
  worker: "/services/new?type=background_worker",
  pserv: "/services/new?type=private_service",
  static: "/services/new?type=static_site",
  cron: "/services/new?type=cron_job",
  d: "/?new=database",
  r: "/keyvalue/new",
};

/** Splat → path parts, tolerating a leading slash and empties — the one
 *  normalization every splat alias route shares. */
export function splatParts(splat: string | undefined): string[] {
  return (splat ?? "").split("/").filter(Boolean);
}

/**
 * Redirect a Render-shaped alias path to its bex canonical route, preserving
 * any sub-path (`/web/srv-x/deploys/dep-y` → `/services/srv-x/deploys/dep-y`)
 * plus the query string and hash. A bare segment with no id (`/web`) has no
 * Render meaning — send it to the workspace overview. `/{segment}/new` is the
 * segment's create URL (RENDER_CREATE_LANDINGS above).
 */
export function redirectRenderAlias(
  segment: RenderAliasSegment,
  splat: string | undefined,
  location: ParsedLocation,
): never {
  const rest = (splat ?? "").replace(/^\/+/, "");
  const path =
    rest === "new"
      ? RENDER_CREATE_LANDINGS[segment]
      : rest
        ? `${RENDER_ALIAS_TARGETS[segment]}/${rest}`
        : "/";
  redirectPreservingSuffix(path, location);
}

/**
 * Throw a replace-redirect to `path`, carrying the incoming query string and
 * hash along (the alias contract: `/web/srv-x/logs?level=error` keeps its
 * filter). A landing that already carries its own query (`/?new=database`)
 * folds the incoming one in with `&`.
 */
export function redirectPreservingSuffix(
  path: string,
  location: ParsedLocation,
): never {
  let suffix = location.href.slice(location.pathname.length);
  if (suffix.startsWith("?") && path.includes("?")) {
    suffix = "&" + suffix.slice(1);
  }
  throw redirect({ href: `${path}${suffix}`, replace: true });
}
