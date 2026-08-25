// Deploy-status → badge/label mapping shared by the Events tab (deploy history
// rows) and the deploy detail page's header (w9/m1) — one place so the two
// surfaces can't drift on what a status badge looks like.

export type DeployBadgeVariant =
  | "default"
  | "secondary"
  | "destructive"
  | "outline";

/** Terminal values in Render's eleven-state deploy vocabulary. */
const TERMINAL_STATUSES = new Set([
  "build_failed",
  "canceled",
  "deactivated",
  "live",
  "pre_deploy_failed",
  "update_failed",
]);

export function isTerminalDeployStatus(status: string): boolean {
  return TERMINAL_STATUSES.has(status);
}

const CANCELABLE_STATUSES = new Set([
  "created",
  "queued",
  "build_in_progress",
  "pre_deploy_in_progress",
  "update_in_progress",
]);

export function isCancelableDeployStatus(status: string): boolean {
  return CANCELABLE_STATUSES.has(status);
}

/**
 * Rollback targets are deploys that reached live and retained a resolved image.
 * bex-api exposes those rows as either the current `live` deploy or a historical
 * `deactivated` deploy; the server performs the final image-availability check.
 */
export function isRollbackableDeployStatus(status: string): boolean {
  return status === "live" || status === "deactivated";
}

export function deployStatusVariant(status: string): DeployBadgeVariant {
  switch (status) {
    case "live":
    case "succeeded":
      return "default";
    case "update_in_progress":
      return "secondary";
    case "update_failed":
    case "build_failed":
    case "pre_deploy_failed":
      return "destructive";
    case "canceled":
    case "deactivated":
      return "outline";
    default:
      return "secondary";
  }
}

export function deployStatusKey(status: string): string {
  switch (status) {
    case "created":
      return "deploys.statusCreated";
    case "queued":
      return "deploys.statusQueued";
    case "build_in_progress":
      return "deploys.statusBuildInProgress";
    case "build_failed":
      return "deploys.statusBuildFailed";
    case "pre_deploy_in_progress":
      return "deploys.statusPreDeployInProgress";
    case "pre_deploy_failed":
      return "deploys.statusPreDeployFailed";
    case "live":
      return "deploys.statusLive";
    case "update_in_progress":
      return "deploys.statusUpdateInProgress";
    case "update_failed":
      return "deploys.statusUpdateFailed";
    case "canceled":
      return "deploys.statusCanceled";
    case "deactivated":
      return "deploys.statusDeactivated";
    default:
      return "deploys.statusUnknown";
  }
}

// The pre-deploy step's own status line (w1/m33), shown under the deploy badge
// so a migration failure reads distinctly from a health-check failure. Only
// running/succeeded/failed carry a label; "" (no pre-deploy step) shows nothing.
export function preDeployStatusKey(status: string): string | null {
  switch (status) {
    case "running":
      return "services.eventsPreDeployRunning";
    case "succeeded":
      return "services.eventsPreDeploySucceeded";
    case "failed":
      return "services.eventsPreDeployFailed";
    default:
      return null;
  }
}

// Deploy.trigger's plain-string values (store.Trigger* — "create"|"api"|
// "deploy_hook"|"blueprint"|"rollback"). "rollback" is deliberately absent
// here: the deploy header/list render it via `deploys.triggerRollback`,
// interpolating the restored deploy's id (rollbackOf) — a param this pure
// enum→key lookup has no way to carry — so that case stays in the caller. An
// unrecognized value returns null so the caller can fall back to rendering it
// verbatim.
export function deployTriggerKey(trigger: string): string | null {
  switch (trigger) {
    case "create":
      return "deploys.triggerCreate";
    case "api":
      return "deploys.triggerApi";
    case "deploy_hook":
      return "deploys.triggerDeployHook";
    case "blueprint":
      return "deploys.triggerBlueprint";
    case "new_commit":
      return "deploys.triggerNewCommit";
    case "config_change":
      return "deploys.triggerConfigChange";
    default:
      return null;
  }
}
