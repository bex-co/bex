import {
  Ban,
  CheckCircle2,
  CircleDot,
  Globe,
  GitBranch,
  HardDrive,
  Hammer,
  History,
  Moon,
  PauseCircle,
  PlayCircle,
  Sunrise,
  RefreshCcw,
  Rocket,
  Scale,
  Terminal,
  Unplug,
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
  // Idle sleep/wake reads as its own thing, not a paused/played service — the
  // whole point of splitting these out of the suspend pair (w6/m47).
  if (type === "service_hibernated") return <Moon {...iconProps} />;
  if (type === "service_woken") return <Sunrise {...iconProps} />;
  if (
    type === "suspender_removed" ||
    type === "service_resumed" ||
    type === "server_available"
  ) {
    return <PlayCircle {...iconProps} />;
  }
  if (type === "server_restarted") return <RefreshCcw {...iconProps} />;
  // Domain ownership passing its check is the awaited beat of the custom-domain
  // journey (ADR005), so it reads as a globe rather than a generic settings dot.
  if (type === "custom_domain_verified") return <Globe {...iconProps} />;
  // The persistent-disk lifecycle (ADR082): attach/update share the drive glyph,
  // detach unplugs it, restore is a point-in-time rewind.
  if (type === "disk_attached" || type === "disk_updated") {
    return <HardDrive {...iconProps} />;
  }
  if (type === "disk_detached") return <Unplug {...iconProps} />;
  if (type === "disk_restored") return <History {...iconProps} />;
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
