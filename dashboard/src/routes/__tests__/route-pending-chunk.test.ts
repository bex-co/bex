import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import {
  AccountSettingsPageSkeleton,
  WorkspaceSettingsPageSkeleton,
} from "@/common/components/route-skeletons";
import { DeployHistorySkeleton } from "@/features/deploys/components/deploy-history-skeleton";
import {
  OrganizationMembersRouteSkeleton,
  OrganizationSettingsRouteSkeleton,
  OrganizationWorkspacesRouteSkeleton,
} from "@/features/organizations/components/organization-route-skeletons";
import { SandboxGroupsListRouteSkeleton } from "@/features/sandbox-groups/components/sandbox-group-skeletons";
import {
  BuildDetailRouteSkeleton,
  BuildSourceBuildsTabSkeleton,
  BuildSourceDetailRouteSkeleton,
  BuildSourceEnvironmentTabSkeleton,
  BuildSourceServicesTabSkeleton,
  BuildSourceSettingsTabSkeleton,
  BuildSourcesListRouteSkeleton,
  CreateBuildSourceRouteSkeleton,
} from "@/features/build-sources/components/build-source-skeletons";
import { Route as AccountSettingsRoute } from "../settings";
import { Route as WorkspaceSettingsRoute } from "../workspace.settings";
import { Route as DeploysRoute } from "../services.$serviceId.deploys.index";
import { Route as DatabaseRoute } from "../databases.$databaseId";
import { Route as WebhookRoute } from "../webhook.$webhookId";
import { Route as ServiceRoute } from "../services.$serviceId";
import { Route as OrganizationWorkspacesRoute } from "../org.$orgId.workspaces";
import { Route as OrganizationMembersRoute } from "../org.$orgId.members";
import { Route as OrganizationSettingsRoute } from "../org.$orgId.settings";
import { Route as SandboxGroupsRoute } from "../sandbox-groups";
import { Route as SandboxGroupDetailRoute } from "../sandbox-groups_.$sandboxGroupId";
import { Route as SandboxGroupSandboxesRoute } from "../sandbox-groups_.$sandboxGroupId.index";
import { Route as SandboxGroupSnapshotsRoute } from "../sandbox-groups_.$sandboxGroupId.snapshots";
import { Route as SandboxGroupImagesRoute } from "../sandbox-groups_.$sandboxGroupId.images";
import { Route as SandboxGroupSettingsRoute } from "../sandbox-groups_.$sandboxGroupId.settings";
import {
  ImagesPageSkeleton,
  SandboxGroupSettingsSkeleton,
  SandboxesPageSkeleton,
  SnapshotsPageSkeleton,
} from "@/features/sandbox-group-pages/components/group-page-skeletons";
import { Route as BuildSourcesRoute } from "../build-sources";
import { Route as BuildSourcesNewRoute } from "../build-sources_.new";
import { Route as BuildSourceDetailRoute } from "../build-sources_.$buildSourceId";
import { Route as BuildSourceBuildsRoute } from "../build-sources_.$buildSourceId.index";
import { Route as BuildSourceBuildDetailRoute } from "../build-sources_.$buildSourceId.builds.$buildId";
import { Route as BuildSourceServicesRoute } from "../build-sources_.$buildSourceId.services";
import { Route as BuildSourceEnvironmentRoute } from "../build-sources_.$buildSourceId.environment";
import { Route as BuildSourceSettingsRoute } from "../build-sources_.$buildSourceId.settings";

/**
 * Where a route's `pendingComponent` lands when the router plugin code-splits
 * the route (w3/066). vitest.config.ts runs the same splitter, with the same
 * groupings, as the production build, so `Route.options` here is what the
 * browser gets.
 *
 * A pending component split into the route's lazy chunk suspends with that
 * chunk, and the router shows its generic spinner instead: the skeleton never
 * paints on a cold navigation. Kept in the route's reference file, it is loaded
 * with the route tree and paints while the chunk downloads.
 */
const isLazy = (component: unknown): boolean =>
  typeof (component as { preload?: unknown } | undefined)?.preload ===
  "function";

describe("route pending components and code splitting (w3/066)", () => {
  it.each([
    [
      "/workspace/settings",
      WorkspaceSettingsRoute,
      WorkspaceSettingsPageSkeleton,
    ],
    ["/settings", AccountSettingsRoute, AccountSettingsPageSkeleton],
    ["/services/$serviceId/deploys/", DeploysRoute, DeployHistorySkeleton],
    // w7/m464: the org pages' skeletons live in a skeleton-only module.
    [
      "/org/$orgId/workspaces",
      OrganizationWorkspacesRoute,
      OrganizationWorkspacesRouteSkeleton,
    ],
    [
      "/org/$orgId/members",
      OrganizationMembersRoute,
      OrganizationMembersRouteSkeleton,
    ],
    [
      "/org/$orgId/settings",
      OrganizationSettingsRoute,
      OrganizationSettingsRouteSkeleton,
    ],
    // w2/m533: sandbox groups & detail skeletons live in a skeleton-only module.
    [
      "/sandbox-groups",
      SandboxGroupsRoute,
      SandboxGroupsListRouteSkeleton,
    ],
    // w1/m658: a group's pages draw their skeletons from a skeleton-only module.
    [
      "/sandbox-groups/$sandboxGroupId/",
      SandboxGroupSandboxesRoute,
      SandboxesPageSkeleton,
    ],
    [
      "/sandbox-groups/$sandboxGroupId/snapshots",
      SandboxGroupSnapshotsRoute,
      SnapshotsPageSkeleton,
    ],
    [
      "/sandbox-groups/$sandboxGroupId/images",
      SandboxGroupImagesRoute,
      ImagesPageSkeleton,
    ],
    [
      "/sandbox-groups/$sandboxGroupId/settings",
      SandboxGroupSettingsRoute,
      SandboxGroupSettingsSkeleton,
    ],
    // w2/m613–m614: build sources list, create & detail skeletons live in a skeleton-only module.
    [
      "/build-sources",
      BuildSourcesRoute,
      BuildSourcesListRouteSkeleton,
    ],
    [
      "/build-sources/new",
      BuildSourcesNewRoute,
      CreateBuildSourceRouteSkeleton,
    ],
    [
      "/build-sources/$buildSourceId",
      BuildSourceDetailRoute,
      BuildSourceDetailRouteSkeleton,
    ],
    [
      "/build-sources/$buildSourceId/",
      BuildSourceBuildsRoute,
      BuildSourceBuildsTabSkeleton,
    ],
    [
      "/build-sources/$buildSourceId/builds/$buildId",
      BuildSourceBuildDetailRoute,
      BuildDetailRouteSkeleton,
    ],
    [
      "/build-sources/$buildSourceId/services",
      BuildSourceServicesRoute,
      BuildSourceServicesTabSkeleton,
    ],
    [
      "/build-sources/$buildSourceId/environment",
      BuildSourceEnvironmentRoute,
      BuildSourceEnvironmentTabSkeleton,
    ],
    [
      "/build-sources/$buildSourceId/settings",
      BuildSourceSettingsRoute,
      BuildSourceSettingsTabSkeleton,
    ],
  ] as const)(
    "%s keeps its skeleton out of the page's lazy chunk",
    (_path, route, skeleton) => {
      // The page itself is still split, so this is the splitter's output.
      expect(isLazy(route.options.component)).toBe(true);
      expect(route.options.pendingComponent).toBe(skeleton);
    },
  );

  it.each([
    ["/databases/$databaseId", DatabaseRoute],
    // w1/m658: the group layout draws its pending frame with the layout.
    ["/sandbox-groups/$sandboxGroupId", SandboxGroupDetailRoute],
    ["/webhook/$webhookId", WebhookRoute],
    ["/services/$serviceId", ServiceRoute],
  ] as const)(
    "%s keeps its page-as-pending frame in the page's chunk",
    (_path, route) => {
      // Importing the module at all is the old ReferenceError guard: a frame
      // shared by both options and split apart left an unbound identifier.
      expect(isLazy(route.options.component)).toBe(true);
      expect(isLazy(route.options.pendingComponent)).toBe(true);
    },
  );

  const ROUTES_DIR = join(import.meta.dirname, "..");
  const routeSources = readdirSync(ROUTES_DIR)
    .filter((file) => /\.tsx?$/.test(file))
    .map((file) => ({
      file,
      source: readFileSync(join(ROUTES_DIR, file), "utf8"),
    }));
  const groupsPendingWithPage = (source: string) =>
    /codeSplitGroupings:\s*\[\s*\[[^\]]*"pendingComponent"/.test(source);

  it("never puts a route-skeletons.tsx skeleton back into the page's chunk", () => {
    // route-skeletons.tsx is already in the entry chunk (the root's
    // EmailVerificationGate imports it), so leaving these skeletons in the
    // route tree costs nothing.
    const offenders = routeSources
      .filter(({ source }) => {
        const pending = source.match(
          /pendingComponent:\s*([A-Za-z_$][\w$]*)/,
        )?.[1];
        if (!pending) return false;
        const fromRouteSkeletons = new RegExp(
          `import\\s*\\{[^}]*\\b${pending}\\b[^}]*\\}\\s*from\\s*"@/common/components/route-skeletons"`,
        ).test(source);
        return fromRouteSkeletons && groupsPendingWithPage(source);
      })
      .map(({ file }) => file);

    expect(offenders).toEqual([]);
  });

  it("groups every page-as-pending route with its page", () => {
    const offenders = routeSources
      .filter(({ source }) => {
        const component = source.match(
          /\bcomponent:\s*([A-Za-z_$][\w$]*)/,
        )?.[1];
        const pending = source.match(
          /pendingComponent:\s*([A-Za-z_$][\w$]*)/,
        )?.[1];
        return (
          component !== undefined &&
          component === pending &&
          !groupsPendingWithPage(source)
        );
      })
      .map(({ file }) => file);

    expect(offenders).toEqual([]);
  });
});
