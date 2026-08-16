import {
  Ban,
  CheckCircle2,
  CircleDot,
  GitBranch,
  Hammer,
  PauseCircle,
  PlayCircle,
  RefreshCcw,
  Rocket,
  Scale,
  Terminal,
  XCircle,
} from "lucide-react";

// Presentation mapping for the service activity feed: a service event's
// type/status to its icon, its icon chip colour, and its badge variant. Pure
// functions with no route dependency, kept beside the event vocabulary in
// service-event-catalog.ts rather than inside the route module.

export function EventIcon({
  type,
  status,
  factStatus,
}: {
  type: string;
  status: string;
  factStatus: string;
}) {
  const iconProps = { className: "size-4", "aria-hidden": true } as const;

  if (type === "deploy_started") return <Rocket {...iconProps} />;
  if (type === "build_started") return <Hammer {...iconProps} />;
  if (type === "pre_deploy_started") return <Terminal {...iconProps} />;
  // Lifecycle-step endings (w7/m66) render by their outcome: check / cross / ban.
  if (
    type === "build_ended" ||
    type === "pre_deploy_ended" ||
    type === "job_run_ended"
  ) {
    if (factStatus === "failed") return <XCircle {...iconProps} />;
    if (factStatus === "canceled") return <Ban {...iconProps} />;
    return <CheckCircle2 {...iconProps} />;
  }
  if (type === "branch_deleted") return <GitBranch {...iconProps} />;
  if (type === "image_pull_failed" || type === "server_failed") {
    return <XCircle {...iconProps} />;
  }
  if (type === "deploy_ended") {
    return status === "update_failed" ? (
      <XCircle {...iconProps} />
    ) : (
      <CheckCircle2 {...iconProps} />
    );
  }
  if (type === "suspender_added" || type === "service_suspended") {
    return <PauseCircle {...iconProps} />;
  }
  if (
    type === "suspender_removed" ||
    type === "service_resumed" ||
    type === "server_available"
  ) {
    return <PlayCircle {...iconProps} />;
  }
  if (type === "server_restarted") return <RefreshCcw {...iconProps} />;
  if (
    type === "instance_count_changed" ||
    type === "autoscaling_config_changed" ||
    type === "autoscaling_started" ||
    type === "autoscaling_ended"
  ) {
    return <Scale {...iconProps} />;
  }
  return <CircleDot {...iconProps} />;
}
