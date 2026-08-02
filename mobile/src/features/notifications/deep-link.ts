export const notificationEvents = [
  "deploy_started",
  "deploy_succeeded",
  "deploy_failed",
  "server_failed",
  "cron_failed",
  "agent_needs_decision",
  "agent_pr_ready",
] as const;

export type NotificationEvent = (typeof notificationEvents)[number];
export type NotificationRoute =
  | `/services/srv-${string}`
  | `/databases/dpg-${string}`
  | `/key-values/red-${string}`
  | `/sessions/ags-${string}`;

export type NotificationEnvelope = {
  schema: "bex.notification.v1";
  notificationId: string;
  event: NotificationEvent;
  route: NotificationRoute;
};

const eventSet = new Set<string>(notificationEvents);
const notificationId = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
const routes = [
  /^\/services\/srv-[a-z0-9]{1,64}$/,
  /^\/databases\/dpg-[a-z0-9]{1,64}$/,
  /^\/key-values\/red-[a-z0-9]{1,64}$/,
  /^\/sessions\/ags-[a-z0-9]{1,64}$/,
];

export function parseNotificationEnvelope(
  value: unknown,
): NotificationEnvelope | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort();
  if (keys.join(",") !== "event,notificationId,route,schema") return null;
  if (record.schema !== "bex.notification.v1") return null;
  if (
    typeof record.notificationId !== "string" ||
    !notificationId.test(record.notificationId)
  ) {
    return null;
  }
  if (typeof record.event !== "string" || !eventSet.has(record.event)) {
    return null;
  }
  const route = record.route;
  if (
    typeof route !== "string" ||
    route.includes("?") ||
    route.includes("#") ||
    route.includes("..") ||
    !routes.some((pattern) => pattern.test(route))
  ) {
    return null;
  }
  return record as NotificationEnvelope;
}
