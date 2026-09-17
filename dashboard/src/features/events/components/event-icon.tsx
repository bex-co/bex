import {
  Ban,
  CheckCircle2,
  CircleDot,
  FolderInput,
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
  type LucideIcon,
} from "lucide-react";

// Presentation mapping for the service activity feed: a service event's
// type/status to its icon, its icon chip colour, and its badge variant. Pure
// functions with no route dependency, kept beside the event vocabulary in
// service-event-catalog.ts rather than inside the route module.

const EVENT_ICONS: Record<string, LucideIcon> = {
  deploy_started: Rocket,
  build_started: Hammer,
  pre_deploy_started: Terminal,
  branch_deleted: GitBranch,
  image_pull_failed: XCircle,
  server_failed: XCircle,
  suspender_added: PauseCircle,
  service_suspended: PauseCircle,
  // Idle sleep/wake stays distinct from an explicit suspend/resume (w6/m47).
  service_hibernated: Moon,
  service_woken: Sunrise,
  suspender_removed: PlayCircle,
  service_resumed: PlayCircle,
  server_available: PlayCircle,
  server_restarted: RefreshCcw,
  custom_domain_verified: Globe,
  service_moved: FolderInput,
  disk_created: HardDrive,
  disk_updated: HardDrive,
  disk_deleted: Unplug,
  disk_restored: History,
  instance_count_changed: Scale,
  autoscaling_config_changed: Scale,
  autoscaling_started: Scale,
  autoscaling_ended: Scale,
};

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
  if (type === "deploy_ended") {
    return status === "update_failed" ? (
      <XCircle {...iconProps} />
    ) : (
      <CheckCircle2 {...iconProps} />
    );
  }
  const Icon = Object.hasOwn(EVENT_ICONS, type) ? EVENT_ICONS[type] : CircleDot;
  return <Icon {...iconProps} />;
}
