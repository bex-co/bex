/**
 * How a service event's `reasonCode` is worded (w8/m158).
 *
 * `non_zero_exit` and `oom_killed` were written for a cron run ("The run
 * exited with status 1."). A running service reports the same two codes on
 * `server_failed`: its process crashed or was killed for memory, and the
 * instance was restarted. Those get their own sentences. `early_exit` (the
 * process ended on its own) happens only to a running service, so its one
 * sentence sits at the plain key every reason code must have.
 */
const INSTANCE_EXIT_REASONS = ["non_zero_exit", "oom_killed"];

/**
 * bex-api's `unhealthy` for a failed health check that recorded no result
 * (`unhealthyMessage` in lego/backend/internal/events/rest.go): the event's
 * own sentence already says as much.
 */
export const UNHEALTHY_WITHOUT_RESULT = "The instance failed its health check";

/**
 * What a `server_failed` instance's failing health check got back, in
 * Render's words ("HTTP health check failed with status code 503"), or null
 * when the event records none (w6/m634). Render shows it in English, so it is
 * shown verbatim in every language.
 */
export function healthCheckResult(
  type: string,
  reasonCode: string | null,
  unhealthy: string | null | undefined,
): string | null {
  if (
    type !== "server_failed" ||
    reasonCode !== "readiness_failed" ||
    !unhealthy ||
    unhealthy === UNHEALTHY_WITHOUT_RESULT
  ) {
    return null;
  }
  return unhealthy;
}

/**
 * The translation key for `reasonCode` on an event of `type`; a failed health
 * check with a recorded `result` (healthCheckResult) gets the sentence that
 * names it.
 */
export function eventReasonKey(
  type: string,
  reasonCode: string,
  memoryLimit?: string | null,
  result?: string | null,
): string {
  if (result) {
    return "services.eventsReason.instance.readiness_failed_result";
  }
  if (type !== "server_failed" || !INSTANCE_EXIT_REASONS.includes(reasonCode)) {
    return `services.eventsReason.${reasonCode}`;
  }
  if (reasonCode === "oom_killed" && !memoryLimit) {
    return "services.eventsReason.instance.oom_killed_no_limit";
  }
  return `services.eventsReason.instance.${reasonCode}`;
}

/**
 * A plan's memory limit for people: the API reports a Kubernetes quantity
 * ("512Mi", "2Gi"), which reads as "512 MB" and "2 GB". Anything else is shown
 * as given.
 */
export function memoryLimitLabel(quantity: string): string {
  const match = /^(\d+(?:\.\d+)?)(Mi|Gi)$/.exec(quantity.trim());
  if (!match) return quantity;
  return `${match[1]} ${match[2] === "Mi" ? "MB" : "GB"}`;
}

/**
 * Render's AutoDeployDisabled reasons the API sends (w3/m317, w4/m431): a
 * settings change, a dashboard rollback, or a deploy of a specific commit
 * from the dashboard or a deploy hook's `ref`.
 */
export const AUTO_DEPLOY_DISABLED_REASONS = [
  "setting_change",
  "rollback",
  "manual_deploy",
] as const;

/**
 * The translation key for an auto_deploy_disabled event's reason, or null for
 * a reason this dashboard has no sentence for (the row keeps its title).
 */
export function autoDeployDisabledReasonKey(
  reason: string | null | undefined,
): string | null {
  return reason &&
    (AUTO_DEPLOY_DISABLED_REASONS as readonly string[]).includes(reason)
    ? `services.eventsAutoDeployDisabledReason.${reason}`
    : null;
}

/**
 * Triggers for a drain-guard zero-downtime redeploy (w4/m504).
 */
export const REDEPLOY_TRIGGERS = [
  "node_replacement",
  "node_consolidation",
] as const;

/**
 * The translation key for zero_downtime_redeploy_started's trigger, or null
 * when unrecognized (the row keeps its title).
 */
export function redeployTriggerKey(
  trigger: string | null | undefined,
): string | null {
  return trigger && (REDEPLOY_TRIGGERS as readonly string[]).includes(trigger)
    ? `services.eventsRedeployTrigger.${trigger}`
    : null;
}
