import {
  Bell,
  Bot,
  Boxes,
  CalendarClock,
  CreditCard,
  FolderKanban,
  Layers,
  Network,
  Settings,
  Webhook,
} from "lucide-react";
import type { SidebarNavGroup } from "./sidebar-nav-groups";

// Render parity: one "Projects" entry groups every resource type (services,
// databases, key value) on a single page (`routes/index.tsx`), rather than a
// separate nav item per resource kind — `/databases`/`/keyvalue` still exist as
// routes (detail pages, deep links) but no longer get their own sidebar entry.
// The sidebar sits under the workspace switcher, so every entry in it is
// workspace-scoped — Settings included, pointing at the workspace's own settings
// (team, plan, integrations). Account settings (`/settings`) belong to the
// user, not the workspace, and hang off the header's user menu instead —
// Render's own split.
//
// Grouping mirrors Render's live sidebar (2026-07-16 capture,
// docs/render-artifacts/dashboard-routes.md § Sidebar navigation): an
// unlabeled top group (Projects/Blueprints/Environment Groups), then
// **Integrations** (Webhooks, Notifications — Render also lists
// Observability, a drains non-goal) and **Workspace** (Usage standing in for
// Render's Billing — ADR023 — plus Settings). Render's Networking group
// (Private Links, Dedicated IPs) is omitted entirely: both entries are
// DO_NOT_DO non-goals, and an empty group is worse than none.
export const NAV_GROUPS: SidebarNavGroup[] = [
  {
    items: [
      { labelKey: "common.navProjects", to: "/", icon: FolderKanban },
      { labelKey: "common.navBlueprints", to: "/blueprints", icon: Layers },
      { labelKey: "common.navRouter", to: "/router", icon: Network },
      { labelKey: "common.navAgents", to: "/agents", icon: Bot },
      {
        labelKey: "common.navAutomations",
        to: "/automations",
        icon: CalendarClock,
      },
      { labelKey: "common.navEnvGroups", to: "/env-groups", icon: Boxes },
    ],
  },
  {
    labelKey: "common.navIntegrationsGroup",
    items: [
      { labelKey: "common.navWebhooks", to: "/webhooks", icon: Webhook },
      { labelKey: "common.navNotifications", to: "/notifications", icon: Bell },
    ],
  },
  {
    labelKey: "common.navWorkspaceGroup",
    items: [
      { labelKey: "common.navUsage", to: "/billing", icon: CreditCard },
      {
        labelKey: "common.navSettings",
        to: "/workspace/settings",
        icon: Settings,
      },
    ],
  },
];

/**
 * The destinations a workspace sees: Agents and Automations only behind the
 * agents flag, Router only behind the router flag. The sidebar and the ⌘K
 * palette both read this, so a destination added to NAV_GROUPS is searchable
 * without a second edit and never leaks past its gate (w21/031).
 */
export function visibleNavGroups(
  groups: SidebarNavGroup[],
  flags: { agentsEnabled: boolean; routerEnabled: boolean },
): SidebarNavGroup[] {
  return groups
    .map((group) => ({
      ...group,
      items: group.items.filter(
        (item) =>
          ((item.to !== "/agents" && item.to !== "/automations") ||
            flags.agentsEnabled) &&
          (item.to !== "/router" || flags.routerEnabled),
      ),
    }))
    .filter((group) => group.items.length > 0);
}
