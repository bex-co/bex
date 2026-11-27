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
  "/router": {
    kind: "render",
    owner: "routes/router.tsx",
    shape: "router",
    regions: ["usage windows", "API key actions", "API key table"],
  },
  "/agents": {
    kind: "render",
    owner: "routes/agents.tsx",
    shape: "agents-list",
    regions: ["composer", "session-list"],
  },
  "/automations": {
    kind: "render",
    owner: "routes/automations.tsx",
    shape: "automations-list",
    regions: ["page-header", "scope-tabs", "automation-list"],
  },
  "/automations/new": {
    kind: "render",
    owner: "routes/automations_.new.tsx",
    shape: "automation-create",
    regions: ["details", "schedule", "session", "actions"],
  },
  "/automations/$automationId": {
    kind: "render",
    owner: "routes/automations_.$automationId.tsx",
    shape: "automation-detail",
    regions: ["actions", "settings", "task", "run-history"],
  },
  "/automations/$automationId/edit": {
    kind: "render",
    owner: "routes/automations_.$automationId_.edit.tsx",
    shape: "automation-edit",
    regions: ["details", "schedule", "session", "actions"],
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
  "/build-sources": {
    kind: "render",
    owner: "routes/build-sources.tsx",
    shape: "build-sources-list",
    regions: ["page-header", "filters", "build-sources-list"],
  },
  "/build-sources/new": {
    kind: "render",
    owner: "routes/build-sources_.new.tsx",
    shape: "build-source-create",
    regions: [
      "page-header",
      "source-picker",
      "details",
      "build-settings",
      "link-services",
    ],
  },
  "/build-sources/$buildSourceId": {
    kind: "render",
    owner: "routes/build-sources_.$buildSourceId.tsx",
    shape: "build-source-detail",
    regions: ["page-header", "detail-tabs", "builds-list"],
  },
  "/build-sources/$buildSourceId/": {
    kind: "render",
    owner: "routes/build-sources_.$buildSourceId.index.tsx",
    shape: "build-source-builds",
    regions: ["builds-list"],
  },
  "/build-sources/$buildSourceId/builds/$buildId": {
    kind: "render",
    owner: "routes/build-sources_.$buildSourceId.builds.$buildId.tsx",
    shape: "build-source-build-detail",
    regions: ["build-header", "build-fanout", "build-logs"],
  },
  "/build-sources/$buildSourceId/services": {
    kind: "render",
    owner: "routes/build-sources_.$buildSourceId.services.tsx",
    shape: "build-source-services",
    regions: ["linked-services"],
  },
  "/build-sources/$buildSourceId/environment": {
    kind: "render",
    owner: "routes/build-sources_.$buildSourceId.environment.tsx",
    shape: "build-source-environment",
    regions: ["build-environment"],
  },
  "/build-sources/$buildSourceId/settings": {
    kind: "render",
    owner: "routes/build-sources_.$buildSourceId.settings.tsx",
    shape: "build-source-settings",
    regions: ["build-settings"],
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
  "/deploy": {
    kind: "redirect",
    owner: "routes/deploy.tsx",
    destination: "/blueprints/new",
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
  "/setup/payment": {
    kind: "render",
    owner: "routes/setup.payment.tsx",
    shape: "payment-setup",
    regions: ["language-action", "page-header", "payment-setup-card"],
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
  "/auth/account-deleted": {
    kind: "render",
    owner: "routes/auth.account-deleted.tsx",
    shape: "account-deleted",
    regions: ["language-action", "page-header", "status-card"],
  },
  "/legal/agent-models": {
    kind: "redirect",
    owner: "routes/legal.agent-models.tsx",
    destination: "https://bex.co/terms-of-service#agent-model",
  },
  "/auth/error": {
    kind: "render",
    owner: "routes/auth.error.tsx",
    shape: "auth-error",
    regions: ["language-action", "page-header", "status-card"],
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
  // A Blueprint's pages (w3/m677): the layout's header and the open page's
  // frame; the bare URL and `/resources` are Resources.
  "/blueprints/$blueprintId": {
    kind: "render",
    owner: "routes/blueprints.$blueprintId.tsx",
    shape: "blueprint-detail",
    regions: ["resource-header", "active-page"],
  },
  "/blueprints/$blueprintId/": {
    kind: "render",
    owner: "routes/blueprints.$blueprintId.index.tsx",
    shape: "blueprint-resources",
    regions: ["resources"],
  },
  "/blueprints/$blueprintId/resources": {
    kind: "render",
    owner: "routes/blueprints.$blueprintId.resources.tsx",
    shape: "blueprint-resources",
    regions: ["resources"],
  },
  "/blueprints/$blueprintId/syncs": {
    kind: "render",
    owner: "routes/blueprints.$blueprintId.syncs.tsx",
    shape: "blueprint-syncs",
    regions: ["sync-history"],
  },
  "/blueprints/$blueprintId/settings": {
    kind: "render",
    owner: "routes/blueprints.$blueprintId.settings.tsx",
    shape: "blueprint-settings",
    regions: ["metadata", "manifest", "validation", "disconnect"],
  },
  // Render's `sync[/<id>]` lands on Syncs, any other sub-path on Resources.
  "/blueprints/$blueprintId/$": {
    kind: "redirect",
    owner: "routes/blueprints.$blueprintId.$.tsx",
    destination: "Syncs for a sync's path, else Resources",
  },
  // Render's `/blueprint/<id>[/<page>]` (w3/m677).
  "/blueprint/$": {
    kind: "redirect",
    owner: "routes/blueprint.$.tsx",
    destination: "canonical Blueprint route",
  },
  "/blueprint/new": {
    kind: "render",
    owner: "routes/blueprint.new.tsx",
    shape: "blueprint-create",
    regions: [
      "blueprint-form",
      "page-header",
      "source-picker",
      "settings",
      "preview",
      "actions",
    ],
  },
  "/blueprints/new": {
    kind: "redirect",
    owner: "routes/blueprints.new.tsx",
    destination: "/blueprint/new",
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
    regions: ["resource-header", "active-page"],
  },
  // A Postgres database's pages (w3/m641): the bare URL and `/info` are Info.
  "/databases/$databaseId/": {
    kind: "render",
    owner: "routes/databases.$databaseId.index.tsx",
    shape: "database-info",
    regions: [
      "metadata",
      "connection",
      "access-control",
      "sql-console",
      "danger-zone",
    ],
  },
  "/databases/$databaseId/info": {
    kind: "render",
    owner: "routes/databases.$databaseId.info.tsx",
    shape: "database-info",
    regions: [
      "metadata",
      "connection",
      "access-control",
      "sql-console",
      "danger-zone",
    ],
  },
  "/databases/$databaseId/logs": {
    kind: "render",
    owner: "routes/databases.$databaseId.logs.tsx",
    shape: "datastore-logs",
    regions: ["log-filters", "log-lines"],
  },
  "/databases/$databaseId/metrics": {
    kind: "render",
    owner: "routes/databases.$databaseId.metrics.tsx",
    shape: "database-metrics",
    regions: ["datastore-metrics", "insights"],
  },
  "/databases/$databaseId/plan": {
    kind: "render",
    owner: "routes/databases.$databaseId.plan.tsx",
    shape: "database-plan",
    regions: ["plan", "high-availability"],
  },
  "/databases/$databaseId/apps": {
    kind: "render",
    owner: "routes/databases.$databaseId.apps.tsx",
    shape: "database-apps",
    regions: ["apps"],
  },
  "/databases/$databaseId/recovery": {
    kind: "render",
    owner: "routes/databases.$databaseId.recovery.tsx",
    shape: "database-recovery",
    regions: ["recovery"],
  },
  // Render's guided version-upgrade page, which Info's version row opens
  // (w3/m676).
  "/databases/$databaseId/version-upgrade": {
    kind: "render",
    owner: "routes/databases.$databaseId.version-upgrade.tsx",
    shape: "database-version-upgrade",
    regions: ["versions"],
  },
  // Render's sub-paths with no page of their own land where their control is.
  "/databases/$databaseId/$": {
    kind: "redirect",
    owner: "routes/databases.$databaseId.$.tsx",
    destination: "the page holding the sub-path's control, else Info",
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
    regions: ["resource-header", "active-page"],
  },
  // A Key Value instance's pages (w3/m649): the bare URL and `/info` are Info.
  "/keyvalue/$keyValueId/": {
    kind: "render",
    owner: "routes/keyvalue.$keyValueId.index.tsx",
    shape: "keyvalue-info",
    regions: [
      "metadata",
      "maxmemory-policy",
      "persistence-mode",
      "plan",
      "connection",
      "networking",
      "danger-zone",
    ],
  },
  "/keyvalue/$keyValueId/info": {
    kind: "render",
    owner: "routes/keyvalue.$keyValueId.info.tsx",
    shape: "keyvalue-info",
    regions: [
      "metadata",
      "maxmemory-policy",
      "persistence-mode",
      "plan",
      "connection",
      "networking",
      "danger-zone",
    ],
  },
  "/keyvalue/$keyValueId/logs": {
    kind: "render",
    owner: "routes/keyvalue.$keyValueId.logs.tsx",
    shape: "datastore-logs",
    regions: ["log-filters", "log-lines"],
  },
  "/keyvalue/$keyValueId/metrics": {
    kind: "render",
    owner: "routes/keyvalue.$keyValueId.metrics.tsx",
    shape: "keyvalue-metrics",
    regions: ["datastore-metrics"],
  },
  // Render's sub-paths with no page of their own land where their control is.
  "/keyvalue/$keyValueId/$": {
    kind: "redirect",
    owner: "routes/keyvalue.$keyValueId.$.tsx",
    destination: "the page holding the sub-path's control, else Info",
  },
  "/keyvalue/new": {
    kind: "redirect",
    owner: "routes/keyvalue.new.tsx",
    destination: "/new/redis",
  },
  "/new/database": {
    kind: "render",
    owner: "routes/new.database.tsx",
    shape: "database-create",
    regions: [
      "database-form",
      "page-header",
      "name",
      "project-environment",
      "physical-names",
      "version",
      "plan-picker",
      "storage",
      "high-availability",
      "public-access",
      "actions",
    ],
  },
  "/new/project": {
    kind: "redirect",
    owner: "routes/new.project.tsx",
    destination: "/?new=project",
  },
  "/new/redis": {
    kind: "render",
    owner: "routes/new.redis.tsx",
    shape: "keyvalue-create",
    regions: [
      "keyvalue-form",
      "page-header",
      "name",
      "project-environment",
      "version",
      "memory-policy",
      "persistence",
      "plan-picker",
      "public-access",
      "actions",
    ],
  },
  "/new/workspace": {
    kind: "render",
    owner: "routes/new.workspace.tsx",
    shape: "workspace-create",
    regions: [
      "page-header",
      "workspace-details",
      "workspace-plans",
      "payment-method",
      "actions",
    ],
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
    kind: "redirect",
    owner: "routes/services.new.tsx",
    destination: "the type's create path (/web/new, …)",
  },
  "/static/$serviceId": {
    kind: "render",
    owner: "routes/static.$serviceId.tsx",
    shape: "static-active-tab",
    regions: ["service-header", "active-tab"],
  },
  "/web/new": {
    kind: "render",
    owner: "routes/_serviceCreate.tsx",
    shape: "service-create",
    regions: ["page-header", "service-form", "source-picker"],
  },
  "/pserv/new": {
    kind: "render",
    owner: "routes/_serviceCreate.tsx",
    shape: "service-create",
    regions: ["page-header", "service-form", "source-picker"],
  },
  "/worker/new": {
    kind: "render",
    owner: "routes/_serviceCreate.tsx",
    shape: "service-create",
    regions: ["page-header", "service-form", "source-picker"],
  },
  "/cron/new": {
    kind: "render",
    owner: "routes/_serviceCreate.tsx",
    shape: "service-create",
    regions: ["page-header", "service-form", "source-picker"],
  },
  "/static/new": {
    kind: "render",
    owner: "routes/_serviceCreate.tsx",
    shape: "service-create",
    regions: ["page-header", "service-form", "source-picker"],
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
      "inbound-ip-rules",
      "team",
      "danger-zone",
      "section-navigation",
    ],
  },
  // An organization's pages (w7/m464): Owner-only regions (create, invite,
  // the audit log) depend on the caller's role, so only what every role sees
  // is listed.
  "/org/$orgId/workspaces": {
    kind: "render",
    owner: "routes/org.$orgId.workspaces.tsx",
    shape: "organization-workspaces",
    regions: ["page-header", "workspaces-table"],
  },
  "/org/$orgId/members": {
    kind: "render",
    owner: "routes/org.$orgId.members.tsx",
    shape: "organization-members",
    regions: ["page-header", "members-table"],
  },
  "/org/$orgId/settings": {
    kind: "render",
    owner: "routes/org.$orgId.settings.tsx",
    shape: "organization-settings",
    regions: ["page-header", "organization-name"],
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
  "/project/$projectId/observability/logs/application": {
    kind: "render",
    owner: "routes/project.$projectId.observability.logs.application.tsx",
    shape: "project-log-view",
    regions: ["page-header", "log-toolbar", "log-panel"],
  },
  "/project/$projectId/observability/logs/request": {
    kind: "render",
    owner: "routes/project.$projectId.observability.logs.request.tsx",
    shape: "project-log-view",
    regions: ["page-header", "log-toolbar", "log-panel"],
  },
  "/project/$projectId/observability/metrics": {
    kind: "render",
    owner: "routes/project.$projectId.observability.metrics.tsx",
    shape: "project-metrics-view",
    regions: [
      "page-header",
      "metrics-toolbar",
      "metrics-legend",
      "application-metrics",
      "network-metrics",
    ],
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
    shape: "cron-plan",
    regions: ["plan-picker"],
  },
  "/services/$serviceId/redirects": {
    kind: "render",
    owner: "routes/services.$serviceId.redirects.tsx",
    shape: "static-edge-rules",
    regions: ["rules-editor"],
  },
  "/services/$serviceId/runs": {
    kind: "render",
    owner: "routes/services.$serviceId.runs.tsx",
    shape: "cron-runs",
    regions: ["cron-runs-list"],
  },
  "/services/$serviceId/scaling": {
    kind: "redirect",
    owner: "routes/services.$serviceId.scaling.tsx",
    destination: "/services/$serviceId/compute#manual-scaling",
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
    regions: ["general", "source", "section-navigation"],
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
    owner: "routes/project.$projectId._overview.tsx",
    shape: "project-overview",
    regions: ["project-header", "environments"],
  },
  "/project/$projectId/environment/$environmentId": {
    kind: "render",
    owner: "routes/project.$projectId._overview.tsx",
    shape: "project-overview",
    regions: ["project-header", "environments"],
  },
  "/services/$serviceId/": {
    kind: "delegate",
    owner: "routes/services.$serviceId.index.tsx",
    to: "/services/$serviceId",
    reason: "data-dependent redirect",
  },
  "/services/$serviceId/builds/$": {
    kind: "delegate",
    owner: "routes/services.$serviceId.builds.$.tsx",
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
  "/services/$serviceId/compute": {
    kind: "render",
    owner: "routes/services.$serviceId.compute.tsx",
    shape: "service-compute",
    regions: [
      "compute-plan",
      "autoscaling",
      "manual-scaling",
      "recent-metrics",
    ],
  },
  "/services/$serviceId/deploys/$deployId": {
    kind: "render",
    owner: "routes/services.$serviceId.deploys.$deployId.tsx",
    shape: "deploy-detail",
    regions: ["deploy-header", "deploy-logs", "deploy-timeline"],
  },
  "/static/$serviceId/deploys/$deployId": {
    kind: "render",
    owner: "routes/static.$serviceId.deploys.$deployId.tsx",
    shape: "deploy-detail",
    regions: ["deploy-header", "deploy-logs", "deploy-timeline"],
  },
  "/services/$serviceId/deploys/$deployId/rollback": {
    kind: "render",
    owner: "routes/services.$serviceId.deploys.$deployId_.rollback.tsx",
    shape: "deploy-rollback",
    regions: ["deploy-header", "deploy-rollback"],
  },
  "/static/$serviceId/deploys/$deployId/rollback": {
    kind: "render",
    owner: "routes/static.$serviceId.deploys.$deployId_.rollback.tsx",
    shape: "deploy-rollback",
    regions: ["deploy-header", "deploy-rollback"],
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
  "/sandbox-groups": {
    kind: "render",
    owner: "routes/sandbox-groups.tsx",
    shape: "sandbox-groups-list",
    regions: ["page-header", "sandbox-groups-list"],
  },
  // A sandbox group's pages (w1/m658): the layout, Sandboxes (its index),
  // Snapshots, Custom images and Settings, and links by a sandbox's or a
  // snapshot's id, which land on the group's page.
  "/sandbox-groups/$sandboxGroupId": {
    kind: "render",
    owner: "routes/sandbox-groups_.$sandboxGroupId.tsx",
    shape: "sandbox-group-detail",
    regions: ["resource-header", "active-page"],
  },
  "/sandbox-groups/$sandboxGroupId/": {
    kind: "render",
    owner: "routes/sandbox-groups_.$sandboxGroupId.index.tsx",
    shape: "sandbox-group-sandboxes",
    regions: ["sandboxes-toolbar", "run-summary", "sandboxes-table"],
  },
  "/sandbox-groups/$sandboxGroupId/snapshots": {
    kind: "render",
    owner: "routes/sandbox-groups_.$sandboxGroupId.snapshots.tsx",
    shape: "sandbox-group-snapshots",
    regions: ["snapshots-toolbar", "snapshots-table"],
  },
  "/sandbox-groups/$sandboxGroupId/images": {
    kind: "render",
    owner: "routes/sandbox-groups_.$sandboxGroupId.images.tsx",
    shape: "sandbox-group-images",
    regions: ["images-toolbar", "images-table"],
  },
  "/sandbox-groups/$sandboxGroupId/settings": {
    kind: "render",
    owner: "routes/sandbox-groups_.$sandboxGroupId.settings.tsx",
    shape: "sandbox-group-settings",
    regions: ["group-general", "credentials-panel", "danger-zone"],
  },
  "/sandboxes/$sandboxId": {
    kind: "delegate",
    owner: "routes/sandboxes.$sandboxId.tsx",
    to: "/sandbox-groups/$sandboxGroupId/",
    reason: "data-dependent redirect",
  },
  "/sandbox-snapshots/$snapshotId": {
    kind: "delegate",
    owner: "routes/sandbox-snapshots.$snapshotId.tsx",
    to: "/sandbox-groups/$sandboxGroupId/snapshots",
    reason: "data-dependent redirect",
  },
} satisfies { [Path in RoutePath]: RouteSkeletonDisposition };

export type RouteSkeletonManifest = typeof ROUTE_SKELETON_MANIFEST;
