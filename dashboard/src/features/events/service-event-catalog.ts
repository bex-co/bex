export interface ServiceEventGroup {
  key: string;
  types: string[];
}

// Render-shaped presentation order for every service-event type bex can emit.
// The first four groups mirror Render's Events filter; bex-only configuration
// facts remain available under Configuration instead of becoming unfilterable.
//
// This catalog governs GROUPING AND LABELS ONLY, never visibility (w6/m122). The
// Events tab renders whatever the API returns and uses this to decide how a row
// is titled and where its checkbox sits, so a type added to the backend
// vocabulary before it is added here degrades to a generic label rather than
// disappearing. `backend-vocabulary.test.ts` still fails the build on that gap —
// fail-open is the runtime behaviour, not permission to drift.
//
// The four disk_* types sit under Configuration rather than in a group of their
// own: all four come from apps.AddDisk / apps.UpdateDisk / apps.DeleteDisk /
// apps.RestoreDiskSnapshot — accepted user intent against the service's own
// settings, exactly like every other member of this group — and the 2026-07-18
// Render filter capture (docs/render-artifacts/service-events.md) has no disk
// group to mirror, so inventing a sixth group would diverge from the rule that
// the first four groups track Render and Configuration holds the rest.
export const SERVICE_EVENT_GROUPS: ServiceEventGroup[] = [
  {
    key: "deploy",
    types: [
      "deploy_started",
      "deploy_ended",
      "build_started",
      "build_ended",
      "pre_deploy_started",
      "pre_deploy_ended",
      "image_pull_failed",
      "cron_job_run_started",
      "cron_job_run_ended",
      "job_started",
      "job_run_ended",
      "job_canceled",
    ],
  },
  {
    key: "serviceStatus",
    types: [
      "suspender_removed",
      "suspender_added",
      "server_failed",
      "server_restarted",
      "service_resumed",
      "service_suspended",
      "service_hibernated",
      "service_woken",
      "server_available",
    ],
  },
  {
    key: "scaling",
    types: [
      "autoscaling_started",
      "autoscaling_ended",
      "autoscaling_config_changed",
      "instance_count_changed",
      "branch_changed",
      "branch_deleted",
      "commit_ignored",
      "plan_changed",
    ],
  },
  {
    key: "maintenanceMode",
    types: ["maintenance_mode_enabled", "maintenance_mode_uri_updated"],
  },
  {
    key: "configuration",
    types: [
      "env_vars_changed",
      "service_environment_changed",
      "service_moved",
      "env_group_linked",
      "env_group_unlinked",
      "auto_deploy_enabled",
      "auto_deploy_disabled",
      "auto_deploy_changed",
      "idle_timeout_changed",
      "root_directory_changed",
      "dockerfile_path_changed",
      "build_filter_changed",
      "commands_changed",
      "source_changed",
      "display_name_changed",
      "pre_deploy_command_changed",
      "max_shutdown_delay_changed",
      "publish_path_changed",
      "routes_changed",
      "headers_changed",
      "custom_domain_added",
      "custom_domain_removed",
      "custom_domain_verified",
      "disk_created",
      "disk_updated",
      "disk_deleted",
      "disk_restored",
      "deploy_hook_regenerated",
      "notify_on_fail_changed",
      "subdomain_policy_changed",
      "ip_allow_list_changed",
    ],
  },
];

export const SERVICE_EVENT_TYPES = [
  ...new Set(SERVICE_EVENT_GROUPS.flatMap((group) => group.types)),
];

const LABEL_KEYS: Record<string, string> = {
  deploy_started: "services.eventsTypeDeployStarted",
  deploy_ended: "services.eventsTypeDeployFinished",
  build_started: "services.eventsTypeBuildStarted",
  build_ended: "services.eventsTypeBuildEnded",
  pre_deploy_started: "services.eventsTypePreDeployStarted",
  pre_deploy_ended: "services.eventsTypePreDeployEnded",
  image_pull_failed: "services.eventsTypeImagePullFailed",
  cron_job_run_started: "services.eventsTypeCronRunStarted",
  cron_job_run_ended: "services.eventsTypeCronRunFinished",
  job_started: "services.eventsTypeJobStarted",
  job_run_ended: "services.eventsTypeJobRunEnded",
  job_canceled: "services.eventsTypeJobCanceled",
  suspender_added: "services.eventsTypeSuspended",
  suspender_removed: "services.eventsTypeResumed",
  server_failed: "services.eventsTypeInstanceFailed",
  server_restarted: "services.eventsTypeRestarted",
  service_suspended: "services.eventsTypeServiceSuspended",
  service_resumed: "services.eventsTypeServiceResumed",
  service_hibernated: "services.eventsTypeServiceHibernated",
  service_woken: "services.eventsTypeServiceWoken",
  server_available: "services.eventsTypeServiceRecovered",
  autoscaling_started: "services.eventsTypeAutoscalingStarted",
  autoscaling_ended: "services.eventsTypeAutoscalingEnded",
  autoscaling_config_changed: "services.eventsTypeAutoscalingChanged",
  instance_count_changed: "services.eventsTypeInstanceCountChanged",
  branch_changed: "services.eventsTypeBranchChanged",
  branch_deleted: "services.eventsTypeBranchDeleted",
  commit_ignored: "services.eventsTypeCommitIgnored",
  plan_changed: "services.eventsTypePlanChanged",
  maintenance_mode_enabled: "services.eventsTypeMaintenanceModeChanged",
  maintenance_mode_uri_updated: "services.eventsTypeMaintenanceModeUriUpdated",
  env_vars_changed: "services.eventsTypeEnvVarsChanged",
  service_environment_changed: "services.eventsTypeEnvironmentChanged",
  // A project/environment reassignment (w6/m134) — NOT the env-var fact the
  // similarly named type above records.
  service_moved: "services.eventsTypeServiceMoved",
  env_group_linked: "services.eventsTypeEnvGroupLinked",
  env_group_unlinked: "services.eventsTypeEnvGroupUnlinked",
  auto_deploy_enabled: "services.eventsTypeAutoDeployChanged",
  auto_deploy_disabled: "services.eventsTypeAutoDeployChanged",
  auto_deploy_changed: "services.eventsTypeAutoDeployChanged",
  idle_timeout_changed: "services.eventsTypeIdleTimeoutChanged",
  display_name_changed: "services.eventsTypeDisplayNameChanged",
  custom_domain_added: "services.eventsTypeCustomDomainAdded",
  custom_domain_removed: "services.eventsTypeCustomDomainRemoved",
  custom_domain_verified: "services.eventsTypeCustomDomainVerified",
  disk_created: "services.eventsTypeDiskCreated",
  disk_updated: "services.eventsTypeDiskUpdated",
  disk_deleted: "services.eventsTypeDiskDeleted",
  disk_restored: "services.eventsTypeDiskRestored",
  notify_on_fail_changed: "services.eventsTypeNotificationsChanged",
  subdomain_policy_changed: "services.eventsTypeSubdomainPolicyChanged",
  publish_path_changed: "services.eventsTypeStaticSiteChanged",
  routes_changed: "services.eventsTypeStaticSiteChanged",
  headers_changed: "services.eventsTypeStaticSiteChanged",
  root_directory_changed: "services.eventsTypeBuildSettingsChanged",
  dockerfile_path_changed: "services.eventsTypeBuildSettingsChanged",
  build_filter_changed: "services.eventsTypeBuildSettingsChanged",
  commands_changed: "services.eventsTypeBuildSettingsChanged",
  source_changed: "services.eventsTypeBuildSettingsChanged",
  pre_deploy_command_changed: "services.eventsTypeBuildSettingsChanged",
  max_shutdown_delay_changed: "services.eventsTypeBuildSettingsChanged",
  deploy_hook_regenerated: "services.eventsTypeBuildSettingsChanged",
  ip_allow_list_changed: "services.eventsTypeServiceChanged",
};

export function serviceEventLabelKey(type: string): string {
  return LABEL_KEYS[type] ?? "services.eventsTypeServiceChanged";
}

// serviceEventHasExplicitLabel distinguishes "catalogued, and deliberately shown
// under the generic label" (ip_allow_list_changed) from "not catalogued at all",
// which serviceEventLabelKey's return value cannot: both yield
// services.eventsTypeServiceChanged. Only the drift guard needs the difference.
export function serviceEventHasExplicitLabel(type: string): boolean {
  return Object.hasOwn(LABEL_KEYS, type);
}
