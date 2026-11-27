import { describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type AnyRoute,
} from "@tanstack/react-router";
import { Route as EnvGroupAlias } from "../env-group.$";
import { Route as EnvGroupsNew } from "../env-groups_.new";
import { Route as NewEnvGroup } from "../new.env-group";
import { Route as SandboxGroupAlias } from "../sandbox-group.$";
import { Route as OrganizationAlias } from "../organization.$";
import { Route as NewSplat } from "../new.$";
import { Route as SlugNew } from "../$serviceTypeSlug.new";
import { Route as TeamAlias } from "../t.$";
import { Route as NetworkAccess } from "../network-access.$";
import { Route as SpendLimit } from "../settings_.spend-limit";
import { Route as ResourceAlias } from "../resource.$";
import { Route as BuildSourceBuilds } from "../build-sources_.$buildSourceId.builds.index";

// w3/m705: Render's resource and create URLs open bex's pages. The real alias
// routes' beforeLoad runs in a router of their own; every landing is one
// catch-all stub, so the test reads where each URL ends up, query and hash
// included. The literal routes that answer before a splat (`/new/database`,
// `/web/new`, …) are stubs too.

const ALIASES: Array<[string, AnyRoute]> = [
  ["/env-group/$", EnvGroupAlias as unknown as AnyRoute],
  ["/env-groups/new", EnvGroupsNew as unknown as AnyRoute],
  ["/new/env-group", NewEnvGroup as unknown as AnyRoute],
  ["/sandbox-group/$", SandboxGroupAlias as unknown as AnyRoute],
  ["/organization/$", OrganizationAlias as unknown as AnyRoute],
  ["/new/$", NewSplat as unknown as AnyRoute],
  ["/$serviceTypeSlug/new", SlugNew as unknown as AnyRoute],
  ["/t/$", TeamAlias as unknown as AnyRoute],
  ["/network-access/$", NetworkAccess as unknown as AnyRoute],
  ["/settings/spend-limit", SpendLimit as unknown as AnyRoute],
  ["/resource/$", ResourceAlias as unknown as AnyRoute],
  [
    "/build-sources/$buildSourceId/builds/",
    BuildSourceBuilds as unknown as AnyRoute,
  ],
];

const LITERALS = [
  "/new/database",
  "/new/redis",
  "/new/project",
  "/new/workspace",
  "/web/new",
  "/worker/new",
  "/blueprint/new",
  "/env-groups",
  "/env-groups/$groupId",
];

async function land(href: string) {
  const root = createRootRoute();
  // The file Routes' options are typed for the generated tree, not this one.
  const aliases = ALIASES.map(
    ([path, route]) =>
      createRoute({
        getParentRoute: () => root,
        path,
        validateSearch: route.options.validateSearch as never,
        beforeLoad: route.options.beforeLoad as never,
      }) as unknown as AnyRoute,
  );
  const stubs = [...LITERALS, "/$"].map(
    (path) =>
      createRoute({
        getParentRoute: () => root,
        path,
        component: () => null,
      }) as unknown as AnyRoute,
  );
  const router = createRouter({
    routeTree: root.addChildren([...aliases, ...stubs]),
    history: createMemoryHistory({ initialEntries: [href] }),
    context: { client: {} as never, session: {} as never },
  });
  await router.load();
  return router;
}

async function landing(href: string): Promise<string> {
  const router = await land(href);
  const { pathname, searchStr, hash } = router.state.location;
  return `${pathname}${searchStr}${hash ? `#${hash}` : ""}`;
}

const SBG = "sbg-dddddddddddddddddddd";
const SBX = "sbx-dddddddddddddddddddd";
const SNP = "snp-dddddddddddddddddddd";

describe("Render's resource and create URLs (w3/m705)", () => {
  it.each([
    // Environment groups.
    ["/env-group/evg-x?a=1#b", "/env-groups/evg-x?a=1#b"],
    ["/env-group/evg-x/anything", "/env-groups/evg-x"],
    ["/env-group/new", "/env-groups?new=env-group"],
    ["/new/env-group?from=cli", "/env-groups?new=env-group&from=cli"],
    ["/env-groups/new", "/env-groups?new=env-group"],
    // Sandbox groups.
    [`/sandbox-group/${SBG}?a=1#b`, `/sandbox-groups/${SBG}?a=1#b`],
    [`/sandbox-group/${SBG}/sandboxes`, `/sandbox-groups/${SBG}`],
    [
      `/sandbox-group/${SBG}/sandboxes/${SBX}/configurations#b`,
      `/sandbox-groups/${SBG}?sandbox=${SBX}&tab=details#b`,
    ],
    [
      `/sandbox-group/${SBG}/snapshots/${SNP}`,
      `/sandbox-groups/${SBG}/snapshots?snapshot=${SNP}`,
    ],
    [
      `/sandbox-group/${SBG}/builds/agent%20runtime?version=v2`,
      `/sandbox-groups/${SBG}/images?image=agent+runtime&version=v2`,
    ],
    [`/sandbox-group/${SBG}/settings`, `/sandbox-groups/${SBG}/settings`],
    ["/sandbox-group/new", "/?new=sandbox-group"],
    // Organizations (Billing is the component's; see organization-alias).
    ["/organization/org-x?a=1#b", "/org/org-x/workspaces?a=1#b"],
    ["/organization/org-x/members", "/org/org-x/members"],
    ["/organization/org-x/settings", "/org/org-x/settings"],
    [
      "/organization/org-x/new/workspace",
      "/org/org-x/workspaces?new=workspace",
    ],
    // Create: Render's `/new`, its slug table on `/<slug>/new`, and
    // `/new/<slug>` read through the same table.
    ["/new", "/web/new"],
    ["/new/WEB?x=1", "/web/new?x=1"],
    ["/new/postgresql", "/new/database"],
    ["/new/kv", "/new/redis"],
    ["/new/blueprints", "/blueprint/new"],
    ["/new/team", "/new/workspace"],
    ["/new/nope", "/web/new"],
    ["/new/database", "/new/database"],
    ["/postgres/new?x=1#h", "/new/database?x=1#h"],
    ["/background-worker/new", "/worker/new"],
    ["/kv/new", "/new/redis"],
    ["/blueprints-nope/new", "/web/new"],
    ["/web/new", "/web/new"],
    // Render's older team scheme opens what /w/ opens.
    ["/t/tea-x/settings?a=1#b", "/w/tea-x/settings?a=1#b"],
    ["/t/tea-x/settings/members", "/w/tea-x/settings/members"],
    // Workspace pages bex keeps as cards.
    ["/network-access?a=1", "/workspace/settings?a=1#inbound-ip-rules"],
    ["/settings/spend-limit", "/workspace/settings#build-pipeline"],
    // `/resource/<id>` by the id's prefix.
    [
      "/resource/srv-dddddddddddddddddddd/logs?l=1#t",
      "/services/srv-dddddddddddddddddddd/logs?l=1#t",
    ],
    [
      "/resource/dpg-dddddddddddddddddddd",
      "/databases/dpg-dddddddddddddddddddd",
    ],
    [`/resource/${SBG}/settings`, `/sandbox-groups/${SBG}/settings`],
    ["/resource/prj-dddddddddddddddddddd", "/"],
    ["/resource/nonsense", "/"],
    // Render's build source Builds page and a build in it.
    ["/build-sources/bds-x/builds?a=1", "/build-sources/bds-x?a=1"],
    [
      "/build-sources/bds-x/builds?build=bld-y",
      "/build-sources/bds-x/builds/bld-y",
    ],
  ])("opens %s at %s", async (from, to) => {
    expect(await landing(from)).toBe(to);
  });

  it.each([
    "/env-group",
    "/sandbox-group",
    "/organization",
    "/resource/wfl-dddddddddddddddddddd",
  ])(
    "answers %s, which names nothing, with the not-found page",
    async (href) => {
      const router = await land(href);
      expect(router.state.matches.at(-1)?.status).toBe("notFound");
      expect(router.state.location.pathname).toBe(href);
    },
  );

  it("replaces the alias in history instead of stacking it", async () => {
    const router = await land("/env-group/evg-x");
    expect(router.history.length).toBe(1);
  });
});
