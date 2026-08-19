import type {
  BlueprintPreviewQuery,
  BlueprintSyncsQuery,
  SyncBlueprintMutation,
  ValidateBlueprintQuery,
} from "@/graphql/definitions";
import type {
  BlueprintPreviewResult,
  BlueprintPreviewValidation,
  BlueprintSyncView,
  BlueprintValidationResult,
  BlueprintView,
  SyncBlueprintResult,
} from "@/features/blueprints/types";
import { nonNull } from "@/common/lib/non-null";

/**
 * Normalizers from the generated GraphQL result types (deeply nullable, per
 * the schema) onto the feature's non-null view models (w1/m79/t002). Every
 * operation selects a different subset of Blueprint fields, so the row input
 * is structural: fields a selection omits fall back to the view default —
 * exactly what consumers of the old hand-written (and dishonestly non-null)
 * document types observed at runtime.
 */
interface BlueprintRowLike {
  id: string | null;
  name: string | null;
  repo?: string | null;
  branch?: string | null;
  path?: string | null;
  autoSync?: boolean | null;
  manifest?: string | null;
  status?: string | null;
  lastSync?: string | null;
  resources?: Array<{
    id: string | null;
    name: string | null;
    type: string | null;
  } | null> | null;
  createdAt?: string | null;
  updatedAt?: string | null;
}

type PreviewData = NonNullable<BlueprintPreviewQuery["blueprintPreview"]>;
type ValidationData = NonNullable<PreviewData["validation"]>;
type PlanData = NonNullable<ValidationData["plan"]>;
type PricingData = NonNullable<ValidationData["estimatedPricing"]>;
type SyncResultData = NonNullable<SyncBlueprintMutation["syncBlueprint"]>;
type SyncRow = NonNullable<
  NonNullable<BlueprintSyncsQuery["blueprintSyncs"]>[number]
>;
type ValidationResultData = NonNullable<
  ValidateBlueprintQuery["validateBlueprint"]
>;

/** Preserve the null-vs-empty distinction the view types carry. */
function strings(items: Array<string | null> | null): string[] | null {
  return items ? items.filter(nonNull) : null;
}

export function toBlueprintView(row: BlueprintRowLike): BlueprintView {
  return {
    id: row.id ?? "",
    name: row.name ?? "",
    repo: row.repo ?? "",
    branch: row.branch ?? "",
    path: row.path ?? "",
    autoSync: row.autoSync ?? false,
    manifest: row.manifest ?? "",
    status: row.status ?? "",
    lastSync: row.lastSync ?? null,
    resources: row.resources
      ? row.resources.filter(nonNull).map((r) => ({
          id: r.id ?? "",
          name: r.name ?? "",
          type: r.type ?? "",
        }))
      : null,
    createdAt: row.createdAt ?? null,
    updatedAt: row.updatedAt ?? null,
  };
}

export function toBlueprintSyncView(row: SyncRow): BlueprintSyncView {
  return {
    id: row.id ?? "",
    commitId: row.commitId ?? "",
    state: row.state ?? "",
    startedAt: row.startedAt ?? null,
    completedAt: row.completedAt ?? null,
  };
}

export function toBlueprintValidationResult(
  v: ValidationResultData,
): BlueprintValidationResult {
  return { valid: v.valid ?? false, errors: strings(v.errors) ?? [] };
}

function toPreviewValidation(v: ValidationData): BlueprintPreviewValidation {
  return {
    valid: v.valid,
    errors: strings(v.errors),
    plan: v.plan ? toPreviewPlan(v.plan) : null,
    estimatedPricing: v.estimatedPricing
      ? toEstimatedPricing(v.estimatedPricing)
      : null,
  };
}

function toPreviewPlan(plan: PlanData) {
  return {
    mode: plan.mode,
    services: strings(plan.services),
    databases: strings(plan.databases),
    keyValue: strings(plan.keyValue),
    envGroups: strings(plan.envGroups),
    syncFalseVars: strings(plan.syncFalseVars),
    totalActions: plan.totalActions,
    actions: plan.actions
      ? plan.actions.filter(nonNull).map((a) => ({
          operation: a.operation ?? "",
          kind: a.kind ?? "",
          name: a.name ?? "",
          sourcePath: a.sourcePath ?? "",
          resourceId: a.resourceId,
          changedFields: a.changedFields
            ? a.changedFields
                .filter(nonNull)
                .map((c) => ({ path: c.path ?? "" }))
            : null,
          message: a.message,
        }))
      : null,
  };
}

function toEstimatedPricing(pricing: PricingData) {
  return {
    totalUsd: pricing.totalUsd,
    lines: pricing.lines
      ? pricing.lines.filter(nonNull).map((line) => ({
          name: line.name ?? "",
          tierLabel: line.tierLabel ?? "",
          monthlyUsd: line.monthlyUsd ?? "",
          instanceUsd: line.instanceUsd,
          storageUsd: line.storageUsd,
          storageGb: line.storageGb,
        }))
      : null,
    variable: pricing.variable
      ? pricing.variable.filter(nonNull).map((v) => ({
          name: v.name ?? "",
          reason: v.reason ?? "",
        }))
      : null,
  };
}

export function toBlueprintPreviewResult(
  preview: PreviewData,
): BlueprintPreviewResult {
  return {
    found: preview.found,
    commitId: preview.commitId,
    error: preview.error,
    validation: preview.validation
      ? toPreviewValidation(preview.validation)
      : null,
  };
}

export function toSyncBlueprintResult(
  result: SyncResultData,
): SyncBlueprintResult {
  return {
    blueprint: result.blueprint ? toBlueprintView(result.blueprint) : null,
    services: result.services
      ? result.services.map((s) =>
          s ? { id: s.id ?? "", name: s.name ?? "" } : null,
        )
      : null,
    databases: strings(result.databases),
  };
}
