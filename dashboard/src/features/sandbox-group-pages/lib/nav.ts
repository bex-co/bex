import { Box, Camera, Layers, Settings } from "lucide-react";
import type {
  SidebarNavGroup,
  SidebarNavItem,
} from "@/common/components/dashboard-layout/sidebar-nav-groups";
import {
  resourcePageNav,
  resourcePageNavItem,
} from "@/common/components/dashboard-layout/resource-page-nav";

// A sandbox group's pages, as Render's group sidebar lists them (w1/m658,
// docs/render-artifacts/sandbox-groups.md § The dashboard): Sandboxes (the
// group's index), Snapshots, Custom images, then Settings. Render's
// `/sandbox-group/<id>` and its `sandboxes/*`, `snapshots/*` and `builds/*`
// are bex's `/sandbox-groups/<id>`, `snapshots` and `images`; a drawer is a
// search parameter of its page.
//
// One source of truth: the group sidebar
// (common/components/dashboard-layout/sandbox-group-sidebar.tsx) draws these
// groups and the group layout names each page with its entry's label.

export type SandboxGroupPagePath = "" | "/snapshots" | "/images" | "/settings";

/** A page's route, typed for links and redirects. */
export function sandboxGroupPageRoute<P extends SandboxGroupPagePath>(page: P) {
  return `/sandbox-groups/$sandboxGroupId${page}` as const;
}

const SANDBOXES = resourcePageNavItem(
  sandboxGroupPageRoute(""),
  "sandboxGroupPages.navSandboxes",
  Box,
);
const SNAPSHOTS = resourcePageNavItem(
  sandboxGroupPageRoute("/snapshots"),
  "sandboxGroupPages.navSnapshots",
  Camera,
);
const IMAGES = resourcePageNavItem(
  sandboxGroupPageRoute("/images"),
  "sandboxGroupPages.navImages",
  Layers,
);
const SETTINGS = resourcePageNavItem(
  sandboxGroupPageRoute("/settings"),
  "sandboxGroupPages.navSettings",
  Settings,
);

export const SANDBOX_GROUP_NAV_GROUPS: readonly SidebarNavGroup[] = [
  { items: [SANDBOXES, SNAPSHOTS, IMAGES] },
  { labelKey: "common.navManageGroup", items: [SETTINGS] },
];

const NAV = resourcePageNav(
  "/sandbox-groups/$sandboxGroupId",
  SANDBOX_GROUP_NAV_GROUPS,
);

/** Whether the sidebar entry `to` is the page `pathname` is on. */
export function isSandboxGroupNavItemActive(
  pathname: string,
  sandboxGroupId: string,
  to: string,
): boolean {
  return NAV.isActive(pathname, sandboxGroupId, to);
}

/**
 * The open page's sidebar label, which is also its heading. Null off the
 * group's pages.
 */
export function sandboxGroupPageLabelKey(
  pathname: string,
  sandboxGroupId: string,
): SidebarNavItem["labelKey"] | null {
  return NAV.labelKey(pathname, sandboxGroupId);
}
