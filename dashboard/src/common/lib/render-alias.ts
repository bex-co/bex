import {
  notFound,
  redirect,
  type ParsedLocation,
} from "@tanstack/react-router";

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
  redirectPreservingSuffix(aliasPath(segment, splat), location);
}

/** The canonical path a Render alias segment's splat names (above). */
function aliasPath(segment: RenderAliasSegment, splat: string | undefined) {
  const rest = (splat ?? "").replace(/^\/+/, "");
  if (rest === "new") return RENDER_CREATE_LANDINGS[segment];
  return rest ? `${RENDER_ALIAS_TARGETS[segment]}/${rest}` : "/";
}

/**
 * Render's create slugs (w3/m705): its `/:serviceTypeSlug/new` route (the
 * `Ez` loader over the `Tz` table in Render's dashboard bundle, read
 * 2026-11-27) opens a service type's create page, sends these other names
 * to their create pages, and anything else to its `/new` chooser. Read
 * case-insensitively.
 */
const RENDER_CREATE_SLUGS: Readonly<Record<string, string>> = {
  web: RENDER_CREATE_LANDINGS.web,
  worker: RENDER_CREATE_LANDINGS.worker,
  pserv: RENDER_CREATE_LANDINGS.pserv,
  static: RENDER_CREATE_LANDINGS.static,
  cron: RENDER_CREATE_LANDINGS.cron,
  "background-worker": "/worker/new",
  "background-workers": "/worker/new",
  blueprint: "/blueprint/new",
  blueprints: "/blueprint/new",
  database: "/new/database",
  postgres: "/new/database",
  postgresql: "/new/database",
  "env-group": "/new/env-group",
  "env-groups": "/new/env-group",
  kv: "/new/redis",
  "key-value": "/new/redis",
  redis: "/new/redis",
};

/**
 * Where Render's `/new` chooser ("Create a new Service") lands. bex has no
 * chooser page of its own yet (w3/154), so it opens the Web Service create
 * page, whose type switcher lists every type.
 */
export const RENDER_NEW_LANDING = "/web/new";

/** The create page Render's `/<slug>/new` opens (RENDER_CREATE_SLUGS). */
export function renderCreateSlugLanding(slug: string | undefined): string {
  return lookup(RENDER_CREATE_SLUGS, slug) ?? RENDER_NEW_LANDING;
}

/** `/new/<page>` names bex serves itself, besides the create slugs; `team`
 *  is Render's older name for a workspace (`/new/team`). */
const NEW_PAGES: Readonly<Record<string, string>> = {
  team: "/new/workspace",
  workspace: "/new/workspace",
  project: "/new/project",
};

/** Where `/new/<slug>` lands: bex's own `/new/<page>`, else the create slug
 *  table, else the `/new` landing. */
export function newPathLanding(slug: string | undefined): string {
  return lookup(NEW_PAGES, slug) ?? renderCreateSlugLanding(slug);
}

/** A case-insensitive own-key read: `constructor` names no create page. */
function lookup<T extends string>(
  table: Readonly<Record<string, T>>,
  key: string | undefined,
): T | undefined {
  const name = key?.toLowerCase();
  return name !== undefined && Object.hasOwn(table, name)
    ? table[name]
    : undefined;
}

/**
 * Render's environment group, `/env-group/<evg-id>` (w3/m705): bex's
 * `/env-groups/<evg-id>`. Render's group has one page, so a sub-path lands on
 * it; `new` is Render's create slug. No id names nothing.
 */
export function envGroupAliasPath([id]: string[]): string | undefined {
  if (!id) return undefined;
  return id === "new"
    ? renderCreateSlugLanding("env-group")
    : `/env-groups/${id}`;
}

/** Render's sandbox drawer tabs → bex's (`configurations` is Render's
 *  Details); Logs and Shell have no bex tab, so the drawer opens on Events. */
const SANDBOX_DRAWER_TAB: Readonly<Record<string, string>> = {
  configurations: "details",
  snapshots: "snapshots",
};

/** `path` with the set values of `query` as its search string. */
function withQuery(
  path: string,
  query: Record<string, string | undefined>,
): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value) params.set(key, value);
  }
  const search = params.toString();
  return search ? `${path}?${search}` : path;
}

/**
 * Render's sandbox group pages, `/sandbox-group/<sbg-id>/…` (w3/m705,
 * docs/render-artifacts/sandbox-groups.md): bex's `/sandbox-groups/<sbg-id>`
 * pages, with a drawer as its page's search parameter (w1/m658). Sandboxes is
 * the index (`sandboxes/<sbx-id>[/<tab>]` opens that sandbox's drawer),
 * `snapshots[/<snp-id>]` the Snapshots page, `builds[/<name>]` Custom images,
 * `settings` Settings, and an unknown sub-path the index. `new` is the create
 * dialog bex's New menu opens. No id names nothing.
 */
export function sandboxGroupAliasPath([id, page, child, tab]: string[]):
  string | undefined {
  if (!id) return undefined;
  if (id === "new") return "/?new=sandbox-group";
  const base = `/sandbox-groups/${id}`;
  switch (page) {
    case "sandboxes":
      return withQuery(base, {
        sandbox: child,
        tab: child ? lookup(SANDBOX_DRAWER_TAB, tab) : undefined,
      });
    case "snapshots":
      return withQuery(`${base}/snapshots`, { snapshot: child });
    case "builds":
      return withQuery(`${base}/images`, { image: child });
    case "settings":
      return `${base}/settings`;
    default:
      return base;
  }
}

/**
 * Render's organization pages, `/organization/<org-id>/…` (w3/m705): bex's
 * `/org/<org-id>/…`, which refuses an org the caller is not in. Workspaces is
 * Render's page for the bare id and any other sub-path; `new/workspace` opens
 * the org's New Workspace dialog. `billing` needs the org's primary workspace
 * and is the route's own. No id names nothing.
 */
export function organizationAliasPath([id, page, sub]: string[]):
  string | undefined {
  if (!id) return undefined;
  const base = `/org/${id}`;
  if (page === "members" || page === "settings") return `${base}/${page}`;
  if (page === "new" && sub === "workspace") {
    return `${base}/workspaces?new=workspace`;
  }
  return `${base}/workspaces`;
}

/** Render ids are a prefix, a hyphen and 20 characters; Render's redirector
 *  sends a shorter one home. */
const RENDER_ID_LENGTH = 24;

/** The id prefixes Render's `/resource/<id>` redirector reads (its `strings`
 *  chunk), with bex's own `blp-` for a Blueprint linked before `exs-`
 *  (w2/m269). Each names the alias segment whose route serves it. */
const RESOURCE_SEGMENTS: Readonly<Record<string, RenderAliasSegment>> = {
  srv: "web",
  crn: "web",
  dpg: "d",
  red: "r",
  exs: "blueprint",
  blp: "blueprint",
};

/** Render's workflow ids: the redirector reads them, bex has none
 *  (DO_NOT_DO: Workflows & tasks). */
const WORKFLOW_PREFIXES = new Set(["wfl", "tsk", "trn", "flw", "frn"]);

/**
 * Render's `/resource/<id>[/<sub-path>]` (w3/m705; its `ResourceRedirector`):
 * the page of the resource the id names, read from its prefix, with the
 * sub-path kept, but for a sandbox or a snapshot, whose link by id opens its
 * drawer as it opens (`/sandboxes/<sbx-id>`, `/sandbox-snapshots/<snp-id>`
 * find the group). As on Render, an id of no known kind, a short one, and a
 * project's go home. A workflow id names nothing bex has.
 */
export function resourceAliasPath([id = "", ...rest]: string[]):
  string | undefined {
  const prefix = id.slice(0, id.indexOf("-"));
  const sub = rest.join("/");
  if (id.length >= RENDER_ID_LENGTH) {
    // Render matches a prefix case-sensitively (`startsWith("srv-")`).
    if (Object.hasOwn(RESOURCE_SEGMENTS, prefix)) {
      const segment = RESOURCE_SEGMENTS[prefix];
      return aliasPath(segment, sub ? `${id}/${sub}` : id);
    }
    switch (prefix) {
      case "whk":
        return sub ? `/webhook/${id}/${sub}` : `/webhook/${id}`;
      case "evg":
        return envGroupAliasPath([id]);
      case "sbg":
        return sandboxGroupAliasPath([id, ...rest]);
      case "sbx":
        return `/sandboxes/${id}`;
      case "snp":
        return `/sandbox-snapshots/${id}`;
    }
    if (WORKFLOW_PREFIXES.has(prefix)) return undefined;
  }
  return "/";
}

/**
 * Redirect to an alias's landing with the query and hash kept, or show the
 * not-found page when the URL names nothing (`/env-group` with no id).
 */
export function redirectOrNotFound(
  path: string | undefined,
  location: ParsedLocation,
): never {
  if (path === undefined) throw notFound();
  redirectPreservingSuffix(path, location);
}

/**
 * Throw a replace-redirect to `path`, carrying the incoming query string and
 * hash along (the alias contract: `/web/srv-x/logs?level=error` keeps its
 * filter). A landing that already carries its own query (`/?new=project`)
 * folds the incoming one in with `&`; one that names its own section
 * (`/workspace/settings#build-pipeline`) keeps it, after the query.
 */
export function redirectPreservingSuffix(
  path: string,
  location: ParsedLocation,
): never {
  const [landing, landingHash] = splitHash(path);
  // The suffix starts at the href's first `?` or `#`, which a path cannot
  // hold unencoded; slicing at the pathname's length misses it whenever the
  // href encodes a character the decoded pathname spells out (`%20`).
  const at = location.href.search(/[?#]/);
  const [query, hash] = splitHash(at < 0 ? "" : location.href.slice(at));
  const joined =
    query.startsWith("?") && landing.includes("?")
      ? `&${query.slice(1)}`
      : query;
  const section = landingHash ?? hash;
  throw redirect({
    href: `${landing}${joined}${section === undefined ? "" : `#${section}`}`,
    replace: true,
  });
}

/** `a#b` → [`a`, `b`]; no `#` → [`a`, undefined]. */
function splitHash(href: string): [string, string | undefined] {
  const at = href.indexOf("#");
  return at < 0 ? [href, undefined] : [href.slice(0, at), href.slice(at + 1)];
}
