/**
 * No-op-safe event sink (w2/m66): forwards to GA4's `gtag` when the document
 * loaded it (VITE_GA_MEASUREMENT_ID, dashboard/AGENTS.md § Analytics), else
 * does nothing.
 *
 * The privacy rule every caller keeps: GA receives only bounded, PII-free
 * labels — matching the SSH gateway's no-identifying-labels rule
 * (docs/ADR035-ssh.md). No resource id or name, no email, no token, no free
 * text and no raw URL ever reaches it: pages are reported by route template
 * (`trackPageView`), and event params pass through `telemetryLabel`.
 *
 * The SSH activation funnel (`ssh_gate_*`) has a server-side counterpart the
 * gate can't see from the browser (whether the eventual SSH auth succeeds): the
 * gateway's `bex_ssh_gateway_authentications_total{result="rejected_key"}` vs
 * `{result="accepted"}` — the metric that root-caused the w2/m65 dead-end. "The
 * gate is working" looks like `rejected_key`'s share falling as `accepted` rises.
 */
export type TelemetryEvent =
  // Acquisition and activation. `sign_up`, `login`, `begin_checkout` and
  // `add_payment_info` are GA4 recommended event names.
  | "sign_up"
  | "login"
  | "email_verified"
  | "begin_checkout"
  | "add_payment_info"
  | "checkout_cancelled"
  | "billing_portal_opened"
  | "workspace_created"
  | "git_connect_started"
  | "ssh_gate_shown"
  | "ssh_gate_cta_clicked"
  // Hosting (the Render side).
  | "service_created"
  | "service_plan_changed"
  | "service_suspended"
  | "service_resumed"
  | "service_deleted"
  | "deploy_triggered"
  | "deploy_rolled_back"
  | "shell_opened"
  | "custom_domain_added"
  | "database_created"
  | "database_plan_changed"
  | "database_deleted"
  | "keyvalue_created"
  | "keyvalue_plan_changed"
  | "keyvalue_deleted"
  | "blueprint_created"
  | "blueprint_synced"
  | "env_group_created"
  | "project_created"
  // Cloud agents (the Devin side).
  | "agent_session_created"
  | "agent_message_sent"
  | "automation_created"
  | "automation_run"
  // Team, access and churn.
  | "member_invited"
  | "invite_accepted"
  | "api_key_created"
  | "ssh_key_added"
  | "webhook_created"
  | "workspace_plan_changed"
  | "workspace_deleted"
  | "account_deleted"
  // Any refused or failed mutation: `action` is the operation name.
  | "action_failed";

export type TelemetryProps = Record<string, string | undefined>;

export function trackEvent(
  event: TelemetryEvent,
  props?: TelemetryProps,
): void {
  if (typeof window === "undefined") return;
  window.gtag?.("event", event, props);
}

const LABEL = /^[A-Za-z0-9_.:-]{1,40}$/;

/** A value fit to send as an event param: a short identifier-shaped string
 * (a plan, a service type, an error code), else nothing. Free text — a name, a
 * prompt, a URL, an email — never matches. */
export function telemetryLabel(value: unknown): string | undefined {
  return typeof value === "string" && LABEL.test(value) ? value : undefined;
}

// What GA4 reads campaign attribution from. Every other query parameter is
// dropped: dashboard URLs carry invite tokens, Kratos flow ids and
// verification codes.
const CAMPAIGN_PARAM = /^(utm_[a-z_]+|gclid|gbraid|wbraid|dclid)$/;

/** The page as GA sees it: the origin plus the route template
 * (`/services/$serviceId/logs`), keeping only campaign parameters. */
export function pageLocation(
  origin: string,
  template: string,
  search: string,
): string {
  const kept = new URLSearchParams();
  for (const [key, value] of new URLSearchParams(search)) {
    if (CAMPAIGN_PARAM.test(key)) kept.append(key, value);
  }
  const query = kept.toString();
  return `${origin}${template}${query ? `?${query}` : ""}`;
}

/** The referrer as GA sees it: another site's origin and path, or this
 * origin's root — a same-origin referrer is a raw dashboard URL. Empty stays
 * empty. */
export function pageReferrer(referrer: string, origin: string): string {
  if (!referrer) return "";
  try {
    const url = new URL(referrer);
    return url.origin === origin
      ? `${origin}/`
      : `${url.origin}${url.pathname}`;
  } catch {
    return "";
  }
}

/**
 * The create pages Render puts outside their section's path, reported in that
 * section, so a section's content group keeps its create page across the move
 * to Render's URLs (the service creates were `/services/new`, Key Value's
 * `/keyvalue/new`, the Blueprint's `/blueprints/new`).
 */
const CREATE_PAGE_GROUPS: Record<string, string> = {
  "/web/new": "services",
  "/pserv/new": "services",
  "/worker/new": "services",
  "/cron/new": "services",
  "/static/new": "services",
  "/new/database": "databases",
  "/new/redis": "keyvalue",
  "/blueprint/new": "blueprints",
};

/** The first path segment of a route template, GA4's `content_group`. */
export function contentGroup(template: string): string {
  return CREATE_PAGE_GROUPS[template] ?? (template.split("/")[1] || "home");
}

/**
 * Report a page view by route template; `location` is `pageLocation`'s result.
 * `set` makes the scrubbed location and title the page context of every later
 * event too — without it gtag falls back to `document.location` and
 * `document.title`, which carry resource ids, names and query strings.
 * `content_group` rides the event: gtag does not send a custom parameter that
 * was only `set`.
 */
export function trackPageView(page: {
  location: string;
  template: string;
  referrer: string;
}): void {
  if (typeof window === "undefined") return;
  window.gtag?.("set", {
    page_location: page.location,
    page_title: page.template,
  });
  window.gtag?.("event", "page_view", {
    page_referrer: page.referrer,
    content_group: contentGroup(page.template),
  });
}
