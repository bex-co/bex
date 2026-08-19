export interface BlueprintResource {
  id: string;
  name: string;
  type: string;
}

export interface BlueprintView {
  id: string;
  name: string;
  repo: string;
  branch: string;
  path: string;
  autoSync: boolean;
  manifest: string;
  status: string;
  lastSync: string | null;
  resources: BlueprintResource[] | null;
  createdAt: string | null;
  updatedAt: string | null;
}

export interface BlueprintSyncView {
  id: string;
  commitId: string;
  state: string;
  startedAt: string | null;
  completedAt: string | null;
}

export interface BlueprintValidationResult {
  valid: boolean;
  errors: string[];
}

export interface BlueprintPreviewPlan {
  mode: string | null;
  services: string[] | null;
  databases: string[] | null;
  keyValue: string[] | null;
  envGroups: string[] | null;
  /** sync:false env var prompt keys ("service/KEY") awaiting first-create values. */
  syncFalseVars: string[] | null;
  totalActions: number | null;
  actions: BlueprintPlanAction[] | null;
}

export interface BlueprintPlanAction {
  operation: string;
  kind: string;
  name: string;
  sourcePath: string;
  resourceId: string | null;
  changedFields: Array<{ path: string }> | null;
  message: string | null;
}

/** One priced row of the estimated-pricing projection (w8/m18). */
export interface BlueprintPricingLine {
  name: string;
  tierLabel: string;
  monthlyUsd: string;
  instanceUsd: string | null;
  storageUsd: string | null;
  storageGb: number | null;
}

/** The known reasons a resource's cost is runtime-dependent — mirrors the backend's VariableCost reasons. */
export type BlueprintVariableReason = "autoscaling" | "multi_instance" | "cron";

/**
 * A resource listed but excluded from the estimated total (runtime-dependent
 * cost). `reason` is the backend's reason string — normally one of
 * BlueprintVariableReason, but kept open (the wire type is a plain string) so
 * an unrecognized reason still renders with the generic "Variable" badge.
 */
export interface BlueprintVariableCost {
  name: string;
  reason: string;
}

/** Always-on monthly cost projection attached to a valid dry-run. */
export interface BlueprintEstimatedPricing {
  totalUsd: string | null;
  lines: BlueprintPricingLine[] | null;
  variable: BlueprintVariableCost[] | null;
}

export interface BlueprintPreviewValidation {
  valid: boolean | null;
  errors: string[] | null;
  plan: BlueprintPreviewPlan | null;
  estimatedPricing: BlueprintEstimatedPricing | null;
}

export interface BlueprintPreviewResult {
  found: boolean | null;
  commitId: string | null;
  error: string | null;
  validation: BlueprintPreviewValidation | null;
}

export interface SyncBlueprintResult {
  blueprint: BlueprintView | null;
  services: Array<{ id: string; name: string } | null> | null;
  databases: string[] | null;
}
