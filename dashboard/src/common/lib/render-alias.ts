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
 * Redirect a Render-shaped alias path to its bex canonical route, preserving
 * any sub-path (`/web/srv-x/deploys/dep-y` → `/services/srv-x/deploys/dep-y`)
 * plus the query string and hash. A bare segment with no id (`/web`) has no
 * Render meaning — send it to the workspace overview.
 */
export function redirectRenderAlias(
  segment: RenderAliasSegment,
  splat: string | undefined,
  location: ParsedLocation,
): never {
  const rest = (splat ?? "").replace(/^\/+/, "");
  const path = rest ? `${RENDER_ALIAS_TARGETS[segment]}/${rest}` : "/";
  const suffix = location.href.slice(location.pathname.length);
  throw redirect({ href: `${path}${suffix}`, replace: true });
}
