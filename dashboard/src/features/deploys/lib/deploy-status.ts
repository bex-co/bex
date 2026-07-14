// Deploy-status → badge/label mapping shared by the Events tab (deploy history
// rows) and the deploy detail page's header (w9/m1) — one place so the two
// surfaces can't drift on what a status badge looks like.

export type DeployBadgeVariant =
  | "default"
  | "secondary"
  | "destructive"
  | "outline";

/** Terminal statuses (store.DeployLive/DeployUpdateFailed/DeployCanceled) — a
 *  deploy in one of these will never change status again, so pollers stop. */
const TERMINAL_STATUSES = new Set(["live", "update_failed", "canceled"]);

export function isTerminalDeployStatus(status: string): boolean {
  return TERMINAL_STATUSES.has(status);
}

export function deployStatusVariant(status: string): DeployBadgeVariant {
  switch (status) {
    case "live":
      return "default";
    case "update_in_progress":
      return "secondary";
    case "update_failed":
      return "destructive";
    case "canceled":
      return "outline";
    default:
      return "secondary";
  }
}

export function deployStatusKey(status: string): string {
  switch (status) {
    case "live":
      return "services.eventsStatusLive";
    case "update_in_progress":
      return "services.eventsStatusInProgress";
    case "update_failed":
      return "services.eventsStatusFailed";
    case "canceled":
      return "services.eventsStatusCanceled";
    default:
      return status;
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
// "rollback"). "rollback" is deliberately absent here: the deploy header
// renders it via `deploys.triggerRollback`, interpolating the restored
// deploy's id (rollbackOf) — a param this pure enum→key lookup has no way to
// carry — so that case stays in the caller. An unrecognized value returns
// null so the caller can fall back to rendering it verbatim.
export function deployTriggerKey(trigger: string): string | null {
  switch (trigger) {
    case "create":
      return "deploys.triggerCreate";
    case "api":
      return "deploys.triggerApi";
    default:
      return null;
  }
}
