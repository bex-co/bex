import type { FileRoutesByFullPath } from "@/routeTree.gen";

type RoutePath = keyof FileRoutesByFullPath;

type RenderingDisposition = {
  kind: "render";
  /** Human-readable owner used in route-sweep evidence and failure output. */
  owner: string;
  /** Stable geometry name emitted as `data-route-skeleton` where applicable. */
  shape: string;
  /** Always-present ready-state regions the pending frame must reserve. */
  regions: readonly [string, ...string[]];
};

type DelegatedDisposition = {
  kind: "delegate";
  owner: string;
  to: RoutePath;
  reason: "layout" | "data-dependent redirect";
};

type RedirectDisposition = {
  kind: "redirect";
  owner: string;
  destination: string;
};

type NonRenderingDisposition = {
  kind: "server" | "not-found";
  owner: string;
};

export type RouteSkeletonDisposition =
  | RenderingDisposition
  | DelegatedDisposition
  | RedirectDisposition
  | NonRenderingDisposition;

/**
 * Exact generated-route ledger for m79. `satisfies` makes this fail closed in
 * both directions: a new `FileRoutesByFullPath` key is missing here, while a
 * removed generated route leaves an excess property. Runtime tests also parse
 * the generated interface so a stale generated file cannot hide drift.
 */
export const ROUTE_SKELETON_MANIFEST = {
  "/": {
    kind: "render",
    owner: "routes/index.tsx",
    shape: "overview",
    regions: ["page-header", "projects", "resources"],
  },
  "/$": { kind: "not-found", owner: "routes/$.tsx" },
  "/agents": {
    kind: "render",
    owner: "routes/agents.tsx",
    shape: "agents-list",
    regions: ["composer", "session-list"],
  },
  "/billing": {
    kind: "render",
    owner: "routes/billing.tsx",
    shape: "billing",
    regions: [
      "page-header",
      "plan",
      "payment-method",
      "included-usage",
      "charges",
      "credit-balance",
      "invoice-history",
      "section-navigation",
    ],
  },
  "/blueprints": {
    kind: "render",
    owner: "routes/blueprints.tsx",
    shape: "blueprints-list",
    regions: ["page-header", "blueprints-table"],
  },
  "/env-groups": {
    kind: "render",
    owner: "routes/env-groups.tsx",
    shape: "env-groups-list",
    regions: ["page-header", "search", "env-groups-table"],
  },
  "/healthz": { kind: "server", owner: "routes/healthz.tsx" },
  "/invite": {
    kind: "render",
    owner: "routes/invite.tsx",
    shape: "invite",
    regions: ["opening"],
  },
  "/login": {
    kind: "redirect",
    owner: "routes/login.tsx",
    destination: "/auth/login",
  },
  "/notifications": {
    kind: "render",
    owner: "routes/notifications.tsx",
    shape: "notifications",
    regions: ["email-notifications", "push-notifications", "web-push"],
  },
  "/register": {
    kind: "redirect",
    owner: "routes/register.tsx",
    destination: "/auth/sign-up",
  },
  "/settings": {
    kind: "render",
    owner: "routes/settings.tsx",
    shape: "account-settings",
    regions: [
      "page-header",
      "account",
      "integrations",
      "access",
      "security",
      "section-navigation",
    ],
  },
  "/usage": {
    kind: "redirect",
    owner: "routes/usage.tsx",
    destination: "/billing",
  },
  "/webhooks": {
    kind: "render",
    owner: "routes/webhooks.tsx",
    shape: "webhooks-list",
    regions: ["webhooks-card"],
  },
  "/agents/$agentSessionId": {
    kind: "render",
    owner: "routes/agents_.$agentSessionId.tsx",
    shape: "agent-session-detail",
    regions: ["session-header", "conversation", "composer"],
  },
  "/api/connected-agents": {
    kind: "server",
    owner: "routes/api.connected-agents.tsx",
  },
  "/api/sessions": { kind: "server", owner: "routes/api.sessions.tsx" },
  "/auth/consent": {
    kind: "render",
    owner: "routes/auth.consent.tsx",
    shape: "oauth-consent",
    regions: ["language-action", "page-header", "consent-card"],
  },
  "/auth/device": {
    kind: "delegate",
    owner: "routes/auth.device.tsx",
    to: "/auth/device/",
    reason: "layout",
  },
  "/auth/forgot-password": {
    kind: "render",
    owner: "routes/auth.forgot-password.tsx",
    shape: "auth-recovery",
    regions: ["language-action", "page-header", "auth-widget"],
  },
  "/auth/login": {
    kind: "render",
    owner: "routes/auth.login.tsx",
    shape: "auth-login",
    regions: ["language-action", "page-header", "auth-widget", "feature-rail"],
  },
  "/auth/logout": {
    kind: "render",
    owner: "routes/auth.logout.tsx",
    shape: "auth-logout",
    regions: ["language-action", "page-header", "logout-card"],
  },
  "/auth/reset-password": {
    kind: "render",
    owner: "routes/auth.reset-password.tsx",
    shape: "account-settings",
    regions: [
      "page-header",
      "account",
      "integrations",
      "access",
      "security",
      "section-navigation",
    ],
  },
  "/auth/sign-up": {
    kind: "render",
    owner: "routes/auth.sign-up.tsx",
    shape: "auth-registration",
    regions: ["language-action", "page-header", "auth-widget", "feature-rail"],
  },
  "/auth/verification": {
    kind: "render",
    owner: "routes/auth.verification.tsx",
    shape: "auth-verification",
    regions: ["language-action", "page-header", "auth-widget"],
  },
  "/blueprints/$blueprintId": {
    kind: "render",
    owner: "routes/blueprints.$blueprintId.tsx",
    shape: "blueprint-detail",
    regions: [
      "page-header",
      "metadata",
      "resources",
      "sync-history",
      "manifest",
      "validation",
    ],
  },
  "/blueprints/new": {
    kind: "render",
    owner: "routes/blueprints.new.tsx",
    shape: "blueprint-create",
    regions: [
      "blueprint-form",
      "form-header",
      "source-picker",
      "settings",
      "preview",
      "actions",
    ],
  },
  "/cron/$": {
    kind: "redirect",
    owner: "routes/cron.$.tsx",
    destination: "canonical service route",
  },
  "/d/$": {
    kind: "redirect",
    owner: "routes/d.$.tsx",
    destination: "canonical datastore route",
  },
  "/databases/$databaseId": {
    kind: "render",
    owner: "routes/databases.$databaseId.tsx",
    shape: "database-detail",
    regions: ["resource-header", "tabs", "active-tab"],
  },
  "/env-groups/$groupId": {
    kind: "render",
    owner: "routes/env-groups_.$groupId.tsx",
    shape: "env-group-detail",
    regions: [
      "page-header",
      "metadata",
      "environment-editor",
      "linked-services",
    ],
  },
  "/keyvalue/$keyValueId": {
    kind: "render",
    owner: "routes/keyvalue.$keyValueId.tsx",
    shape: "keyvalue-detail",
    regions: ["resource-header", "tabs", "active-tab"],
  },
  "/keyvalue/new": {
    kind: "render",
    owner: "routes/keyvalue.new.tsx",
    shape: "keyvalue-create",
    regions: [
      "keyvalue-form",
      "form-header",
      "name",
      "plan-picker",
      "version",
      "memory-policy",
      "persistence",
      "project-environment",
      "public-access",
      "actions",
    ],
  },
  "/new/database": {
    kind: "redirect",
    owner: "routes/new.database.tsx",
    destination: "/?new=database",
  },
  "/new/project": {
    kind: "redirect",
    owner: "routes/new.project.tsx",
    destination: "/?new=project",
  },
  "/new/redis": {
    kind: "redirect",
    owner: "routes/new.redis.tsx",
    destination: "/keyvalue/new",
  },
  "/new/workspace": {
    kind: "render",
    owner: "routes/new.workspace.tsx",
    shape: "workspace-create",
    regions: ["page-header", "workspace-name", "workspace-plans", "actions"],
  },
  "/project/$projectId": {
    kind: "render",
    owner: "routes/project.$projectId.tsx",
    shape: "project-active-child",
    regions: ["active-child"],
  },
  "/pserv/$": {
    kind: "redirect",
    owner: "routes/pserv.$.tsx",
    destination: "canonical private-service route",
  },
  "/r/$": {
    kind: "redirect",
    owner: "routes/r.$.tsx",
    destination: "canonical datastore route",
  },
  "/services/$serviceId": {
    kind: "render",
    owner: "routes/services.$serviceId.tsx",
    shape: "service-active-tab",
    regions: ["service-header", "active-tab"],
  },
  "/services/new": {
    kind: "render",
    owner: "routes/services.new.tsx",
    shape: "service-create",
    regions: [
      "service-form",
      "form-header",
      "service-type",
      "source-picker",
      "settings",
      "plan-picker",
      "project-environment",
      "auto-deploy",
      "environment-variables",
      "secret-files",
      "actions",
    ],
  },
  "/static/$serviceId": {
    kind: "render",
    owner: "routes/static.$serviceId.tsx",
    shape: "static-active-tab",
    regions: ["service-header", "active-tab"],
  },
  "/static/new": {
    kind: "redirect",
    owner: "routes/static.new.tsx",
    destination: "/services/new?type=static_site",
  },
  "/u/$": {
    kind: "redirect",
    owner: "routes/u.$.tsx",
    destination: "canonical user route",
  },
  "/w/$": {
    kind: "render",
    owner: "routes/w.$.tsx",
    shape: "workspace-alias-destination",
    regions: ["destination-page"],
  },
  "/web/$": {
    kind: "redirect",
    owner: "routes/web.$.tsx",
    destination: "canonical web-service route",
  },
  "/webhook/$webhookId": {
    kind: "render",
    owner: "routes/webhook.$webhookId.tsx",
    shape: "webhook-active-tab",
    regions: ["webhook-header", "tabs", "active-tab"],
  },
  "/webhooks/new": {
    kind: "render",
    owner: "routes/webhooks_.new.tsx",
    shape: "webhook-create",
    regions: [
      "page-header",
      "webhook-form",
      "form-header",
      "identity",
      "events",
      "status",
      "actions",
    ],
  },
  "/worker/$": {
    kind: "redirect",
    owner: "routes/worker.$.tsx",
    destination: "canonical worker route",
  },
  "/workspace/settings": {
    kind: "render",
    owner: "routes/workspace.settings.tsx",
    shape: "workspace-settings",
    regions: [
      "page-header",
      "general",
      "team",
      "danger-zone",
      "section-navigation",
    ],
  },
  "/static/": {
    kind: "redirect",
    owner: "routes/static.index.tsx",
    destination: "/",
  },
  "/auth/device/success": {
    kind: "render",
    owner: "routes/auth.device.success.tsx",
    shape: "oauth-device-success",
    regions: ["language-action", "page-header", "terminal"],
  },
  "/billing/$first/$": {
    kind: "redirect",
    owner: "routes/billing_.$first.$.tsx",
    destination: "/billing or /workspace/settings",
  },
  "/project/$projectId/settings": {
    kind: "render",
    owner: "routes/project.$projectId.settings.tsx",
    shape: "project-settings",
    regions: ["page-header", "project-name", "danger-zone"],
  },
  "/services/$serviceId/disk": {
    kind: "render",
    owner: "routes/services.$serviceId.disk.tsx",
    shape: "service-disk",
    regions: ["disk-card"],
  },
  "/services/$serviceId/env": {
    kind: "render",
    owner: "routes/services.$serviceId.env.tsx",
    shape: "service-environment",
    regions: ["page-header", "environment-editor", "environment-groups"],
  },
  "/services/$serviceId/events": {
    kind: "render",
    owner: "routes/services.$serviceId.events.tsx",
    shape: "service-events",
    regions: ["event-feed"],
  },
  "/services/$serviceId/headers": {
    kind: "render",
    owner: "routes/services.$serviceId.headers.tsx",
    shape: "static-edge-rules",
    regions: ["rules-editor"],
  },
  "/services/$serviceId/logs": {
    kind: "render",
    owner: "routes/services.$serviceId.logs.tsx",
    shape: "service-logs",
    regions: ["log-filters", "log-panel"],
  },
  "/services/$serviceId/metrics": {
    kind: "render",
    owner: "routes/services.$serviceId.metrics.tsx",
    shape: "service-metrics",
    regions: ["metrics-filters", "application-metrics", "network-metrics"],
  },
  "/services/$serviceId/plan": {
    kind: "render",
    owner: "routes/services.$serviceId.plan.tsx",
    shape: "service-plan",
    regions: ["plan-picker"],
  },
  "/services/$serviceId/redirects": {
    kind: "render",
    owner: "routes/services.$serviceId.redirects.tsx",
    shape: "static-edge-rules",
    regions: ["rules-editor"],
  },
  "/services/$serviceId/scaling": {
    kind: "render",
    owner: "routes/services.$serviceId.scaling.tsx",
    shape: "service-scaling",
    regions: ["autoscaling", "manual-scaling", "recent-metrics"],
  },
  "/services/$serviceId/settings": {
    kind: "render",
    owner: "routes/services.$serviceId.settings.tsx",
    shape: "service-settings",
    regions: ["general", "section-navigation"],
  },
  "/services/$serviceId/shell": {
    kind: "render",
    owner: "routes/services.$serviceId.shell.tsx",
    shape: "service-shell",
    regions: ["page-header", "web-terminal", "ssh-connection"],
  },
  "/static/$serviceId/env": {
    kind: "render",
    owner: "routes/static.$serviceId.env.tsx",
    shape: "service-environment",
    regions: ["page-header", "environment-editor", "environment-groups"],
  },
  "/static/$serviceId/events": {
    kind: "render",
    owner: "routes/static.$serviceId.events.tsx",
    shape: "service-events",
    regions: ["event-feed"],
  },
  "/static/$serviceId/headers": {
    kind: "render",
    owner: "routes/static.$serviceId.headers.tsx",
    shape: "static-edge-rules",
    regions: ["rules-editor"],
  },
  "/static/$serviceId/metrics": {
    kind: "render",
    owner: "routes/static.$serviceId.metrics.tsx",
    shape: "static-metrics",
    regions: ["metrics-filters", "network-metrics"],
  },
  "/static/$serviceId/redirects": {
    kind: "render",
    owner: "routes/static.$serviceId.redirects.tsx",
    shape: "static-edge-rules",
    regions: ["rules-editor"],
  },
  "/static/$serviceId/settings": {
    kind: "render",
    owner: "routes/static.$serviceId.settings.tsx",
    shape: "static-settings",
    regions: ["general", "section-navigation"],
  },
  "/webhook/$webhookId/settings": {
    kind: "render",
    owner: "routes/webhook.$webhookId.settings.tsx",
    shape: "webhook-settings",
    regions: ["settings-general", "settings-events", "danger-zone"],
  },
  "/auth/device/": {
    kind: "render",
    owner: "routes/auth.device.index.tsx",
    shape: "oauth-device-confirm",
    regions: ["language-action", "page-header", "device-card"],
  },
  "/project/$projectId/": {
    kind: "render",
    owner: "routes/project.$projectId.index.tsx",
    shape: "project-overview",
    regions: ["project-header", "environments"],
  },
  "/services/$serviceId/": {
    kind: "delegate",
    owner: "routes/services.$serviceId.index.tsx",
    to: "/services/$serviceId",
    reason: "data-dependent redirect",
  },
  "/static/$serviceId/": {
    kind: "delegate",
    owner: "routes/static.$serviceId.index.tsx",
    to: "/static/$serviceId",
    reason: "data-dependent redirect",
  },
  "/webhook/$webhookId/": {
    kind: "render",
    owner: "routes/webhook.$webhookId.index.tsx",
    shape: "webhook-activity",
    regions: ["deliveries"],
  },
  "/services/$serviceId/deploys/$deployId": {
    kind: "render",
    owner: "routes/services.$serviceId.deploys.$deployId.tsx",
    shape: "deploy-detail",
    regions: ["deploy-header", "deploy-timeline", "deploy-logs"],
  },
  "/static/$serviceId/deploys/$deployId": {
    kind: "render",
    owner: "routes/static.$serviceId.deploys.$deployId.tsx",
    shape: "deploy-detail",
    regions: ["deploy-header", "deploy-timeline", "deploy-logs"],
  },
  "/services/$serviceId/deploys/": {
    kind: "render",
    owner: "routes/services.$serviceId.deploys.index.tsx",
    shape: "deploys-list",
    regions: ["deploys-table"],
  },
  "/static/$serviceId/deploys/": {
    kind: "render",
    owner: "routes/static.$serviceId.deploys.index.tsx",
    shape: "deploys-list",
    regions: ["deploys-table"],
  },
} satisfies { [Path in RoutePath]: RouteSkeletonDisposition };

export type RouteSkeletonManifest = typeof ROUTE_SKELETON_MANIFEST;
