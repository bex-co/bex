import type { ServiceEventView } from "@/features/events/hooks/use-service-events";

export type EventTimelineFilter = "all" | "deploy" | "lifecycle" | "config";

const DEPLOY_TYPES = new Set([
  "deploy_started",
  "deploy_ended",
  "cron_job_run_started",
  "cron_job_run_ended",
]);
const LIFECYCLE_TYPES = new Set([
  "suspender_added",
  "suspender_removed",
  "server_restarted",
  "instance_count_changed",
  "autoscaling_config_changed",
  "plan_changed",
]);

export function filterTimelineEvents(
  events: ServiceEventView[],
  startTime: string,
  endTime: string,
  filter: EventTimelineFilter,
): ServiceEventView[] {
  const start = Date.parse(startTime);
  const end = Date.parse(endTime);
  return events.filter((event) => {
    const timestamp = Date.parse(event.timestamp ?? "");
    if (!Number.isFinite(timestamp) || timestamp < start || timestamp > end) {
      return false;
    }
    if (filter === "all") return true;
    if (filter === "deploy") return DEPLOY_TYPES.has(event.type ?? "");
    if (filter === "lifecycle") return LIFECYCLE_TYPES.has(event.type ?? "");
    return (
      !DEPLOY_TYPES.has(event.type ?? "") &&
      !LIFECYCLE_TYPES.has(event.type ?? "")
    );
  });
}
