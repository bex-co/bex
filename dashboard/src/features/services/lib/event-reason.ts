/**
 * How a service event's `reasonCode` is worded (w8/m158).
 *
 * `non_zero_exit` and `oom_killed` were written for a cron run ("The run
 * exited with status 1."). A running service reports the same codes, and
 * `early_exit`, on `server_failed`: its process crashed, was killed for
 * memory, or ended on its own, and the instance was restarted. Those get
 * their own sentences.
 */
const INSTANCE_EXIT_REASONS = ["non_zero_exit", "oom_killed", "early_exit"];

/** The translation key for `reasonCode` on an event of `type`. */
export function eventReasonKey(
  type: string,
  reasonCode: string,
  memoryLimit?: string | null,
): string {
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
