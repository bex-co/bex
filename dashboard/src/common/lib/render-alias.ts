import { redirect, type ParsedLocation } from "@tanstack/react-router";

/**
 * Render dashboard URL scheme → bex canonical routes
 * (docs/render-artifacts/dashboard-routes.md). Render's API and its official
 * CLI mint `<dashboard>/{segment}/{id}` deep links (`/web/srv-…`, `/d/dpg-…`,
 * `/r/red-…`, `/blueprint/exs-…`); bex serves them as thin redirect aliases
 * onto the canonical `/services` / `/databases` / `/keyvalue` / `/blueprints`
 * routes. Every service segment
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
  blueprint: "/blueprints",
} as const;

export type RenderAliasSegment = keyof typeof RENDER_ALIAS_TARGETS;

/**
 * Where each segment's CREATE URL lands (w1/m45, w5/m47). Render's New menu puts
 * service creates under the type segment (`/web/new` … — live capture,
 * 2026-07-16), and bex's service create pages live at those same paths; the
 * generic `${target}/${rest}` join would send them to `/services/new`, and
 * `/d/new` to a NONEXISTENT `/databases/new` (bex's Postgres create page is
 * Render's `/new/database`, its Key Value one `/new/redis`, its Blueprint one
 * Render's `/blueprint/new`). Explicit
 * landings make the create shape a contract instead of an accident.
 */
export const RENDER_CREATE_LANDINGS: Record<RenderAliasSegment, string> = {
  web: "/web/new",
  worker: "/worker/new",
  pserv: "/pserv/new",
  static: "/static/new",
  cron: "/cron/new",
  d: "/new/database",
  r: "/new/redis",
  // Its own route (routes/blueprint.new.tsx) answers before the alias does.
  blueprint: "/blueprint/new",
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
 * filter). A landing that already carries its own query (`/?new=project`)
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
