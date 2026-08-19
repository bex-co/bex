export const ROUTE_HEAD_CATEGORIES = [
  "content",
  "inherited-layout",
  "redirect-only",
  "non-html",
  "fallback",
] as const;

export type RouteHeadCategory = (typeof ROUTE_HEAD_CATEGORIES)[number];

/**
 * Exhaustive classification of dashboard/src/routes. The adjacent test compares
 * this manifest to the filesystem so a new route must declare whether it owns,
 * inherits, redirects, omits, or supplies fallback document metadata.
 */
export const ROUTE_HEAD_INVENTORY: Record<
  RouteHeadCategory,
  readonly string[]
> = {
  content: [
    "agents.tsx",
    // MUST keep the trailing-underscore (`agents_`) flat form: `agents.tsx` is a
    // content page with no <Outlet/>, so a nested `agents.$agentSessionId` child
    // would render the list at /agents/{id} instead of the detail page. The
    // underscore opts the detail route out of nesting (cf. env-groups_.$groupId).
    // This inventory entry is the tripwire — renaming back to the nested form
    // fails this test.
    "agents_.$agentSessionId.tsx",
    "auth.consent.tsx",
    "auth.device.success.tsx",
    "auth.device.tsx",
    "auth.forgot-password.tsx",
    "auth.login.tsx",
    "auth.logout.tsx",
    "auth.reset-password.tsx",
    "auth.sign-up.tsx",
    "auth.verification.tsx",
    "billing.tsx",
    "blueprints.$blueprintId.tsx",
    "blueprints.new.tsx",
    "blueprints.tsx",
    "databases.$databaseId.tsx",
    "env-groups.tsx",
    "env-groups_.$groupId.tsx",
    "index.tsx",
    "invite.tsx",
    "keyvalue.$keyValueId.tsx",
    "keyvalue.new.tsx",
    "new.workspace.tsx",
    "notifications.tsx",
    "project.$projectId.settings.tsx",
    "project.$projectId.tsx",
    "services.$serviceId.tsx",
    "services.new.tsx",
    "settings.tsx",
    "static.$serviceId.tsx",
    "webhook.$webhookId.tsx",
    "webhooks.tsx",
    "webhooks_.new.tsx",
    "workspace.settings.tsx",
  ],
  "inherited-layout": [
    "auth.device.index.tsx",
    "project.$projectId.index.tsx",
    "services.$serviceId.deploys.$deployId.tsx",
    "services.$serviceId.deploys.index.tsx",
    "services.$serviceId.env.tsx",
    "services.$serviceId.events.tsx",
    "services.$serviceId.headers.tsx",
    "services.$serviceId.index.tsx",
    "services.$serviceId.logs.tsx",
    "services.$serviceId.metrics.tsx",
    "services.$serviceId.plan.tsx",
    "services.$serviceId.redirects.tsx",
    "services.$serviceId.scaling.tsx",
    "services.$serviceId.settings.tsx",
    "services.$serviceId.shell.tsx",
    "static.$serviceId.deploys.$deployId.tsx",
    "static.$serviceId.deploys.index.tsx",
    "static.$serviceId.env.tsx",
    "static.$serviceId.events.tsx",
    "static.$serviceId.headers.tsx",
    "static.$serviceId.index.tsx",
    "static.$serviceId.metrics.tsx",
    "static.$serviceId.redirects.tsx",
    "static.$serviceId.settings.tsx",
    "webhook.$webhookId.index.tsx",
    "webhook.$webhookId.settings.tsx",
  ],
  "redirect-only": [
    "billing_.$first.$.tsx",
    "cron.$.tsx",
    "d.$.tsx",
    "login.tsx",
    "new.database.tsx",
    "new.project.tsx",
    "new.redis.tsx",
    "pserv.$.tsx",
    "r.$.tsx",
    "register.tsx",
    "static.index.tsx",
    "static.new.tsx",
    "u.$.tsx",
    "usage.tsx",
    "w.$.tsx",
    "web.$.tsx",
    "worker.$.tsx",
  ],
  "non-html": ["api.connected-agents.tsx", "api.sessions.tsx", "healthz.tsx"],
  fallback: ["$.tsx", "__root.tsx"],
};

export const CLASSIFIED_ROUTE_FILES =
  Object.values(ROUTE_HEAD_INVENTORY).flat();
