export const MOBILE_SAFE_ACTIONS = [
  "trigger-deploy",
  "cancel-deploy",
  "rollback-service",
  "restart-service",
  "suspend-service",
  "resume-service",
  "restart-database",
  "suspend-database",
  "resume-database",
  "suspend-key-value",
  "resume-key-value",
  "update-environment-variable",
  "run-cron-job",
  "cancel-cron-run",
] as const;

export type MobileSafeActionId = (typeof MOBILE_SAFE_ACTIONS)[number];
export type SafeActionTargetKind =
  "service" | "deploy" | "database" | "key-value" | "cron-run";

const targetByAction: Record<MobileSafeActionId, SafeActionTargetKind> = {
  "trigger-deploy": "service",
  "cancel-deploy": "deploy",
  "rollback-service": "deploy",
  "restart-service": "service",
  "suspend-service": "service",
  "resume-service": "service",
  "restart-database": "database",
  "suspend-database": "database",
  "resume-database": "database",
  "suspend-key-value": "key-value",
  "resume-key-value": "key-value",
  "update-environment-variable": "service",
  "run-cron-job": "service",
  "cancel-cron-run": "cron-run",
};

export interface SafeActionDefinition<
  Id extends MobileSafeActionId = MobileSafeActionId,
> {
  id: Id;
  targetKind: SafeActionTargetKind;
}

/**
 * The only production registration entry point for mobile operations. Its
 * literal first argument is also inspected by the ADR048 inventory test.
 */
export function defineSafeAction<Id extends MobileSafeActionId>(
  id: Id,
  targetKind: SafeActionTargetKind,
): SafeActionDefinition<Id> {
  if (targetByAction[id] !== targetKind) {
    throw new Error(`mobile action ${id} cannot target ${targetKind}`);
  }
  return Object.freeze({ id, targetKind });
}

export function isMobileSafeAction(value: string): value is MobileSafeActionId {
  return (MOBILE_SAFE_ACTIONS as readonly string[]).includes(value);
}
