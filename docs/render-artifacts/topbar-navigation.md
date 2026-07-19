# Render topbar navigation — research and bex decision

Reviewed 2026-07-18 against the user-supplied Render service-settings target using the repository's authenticated Render walk screenshots and Render's official [dashboard guide](https://render.com/docs/render-dashboard). The relevant retained captures are `.playwright-mcp/render-walk-service-{settings,deploys,events,logs}.png`, `.playwright-mcp/render-walk-projects.png`, and `.playwright-mcp/render-walk-workspace-overview.png` (gitignored).

## Observed Render pattern

Render keeps one topbar across workspace, project, and resource pages. It has two stable zones:

- **Context on the left.** Workspace pages name the current section (for example, Projects). Project pages name the current project. A resource page shows a clickable `Project / Environment / Service` hierarchy. Each hierarchy segment opens a switcher, so the breadcrumb is navigation rather than decorative location text.
- **Global actions on the right.** Workspace-wide Search (`⌘K` / `Ctrl+K`), `+ New`, help, and the account menu stay in the same location on every page. Search jumps directly to resources; `+ New` does not depend on first returning to the workspace overview.

The contextual left sidebar remains important and changes with scope: workspace-wide links at the root, project navigation on project pages, and service navigation on service pages. The topbar complements that sidebar by making cross-resource movement fast. Render documents both the workspace-wide shortcut and the resource breadcrumbs, and its [enhanced-navigation changelog](https://render.com/changelog/enhanced-navigation-in-the-render-dashboard) describes the same division of responsibility.

## Problems in the previous bex shell

The bex sidebar already followed Render's contextual switching, but the shared 48px header contained only the account avatar. That left several gaps:

- no visible page or resource context above the content;
- no direct switch between a service, its environment, and its project;
- no workspace-wide keyboard navigation;
- creation was available only where an individual page happened to add a button;
- the blank header consumed vertical space without helping navigation.

## Implemented bex shape

The shared `DashboardLayout` now supplies the navigation on every authenticated page:

- route-aware page labels for workspace, settings, creation, and resource pages;
- switchable Project / Environment / Service breadcrumbs on service pages and a project switcher on project pages;
- lazy workspace-wide command search over pages, projects, services, Postgres, Key Value, and environment groups, opened by `⌘K` / `Ctrl+K`;
- a persistent create menu for the supported service, Postgres, Key Value, project, and webhook flows;
- help links plus the existing account menu;
- responsive compaction to icons and the current sidebar trigger on narrow screens.

Resource search queries mount only while the command dialog is open. This keeps the persistent topbar from adding API traffic to every normal page load. The topbar retains bex's established compact `h-12` chrome instead of copying Render's dimensions literally.
