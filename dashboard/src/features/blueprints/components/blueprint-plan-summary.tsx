import type { ReactNode } from "react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { EstimatedPricingPanel } from "./estimated-pricing-panel";
import type { en } from "@/i18n";
import type {
  BlueprintEstimatedPricing,
  BlueprintPlanAction,
  BlueprintPreviewPlan,
} from "../types";

/** One named group of planned resources in a blueprint review. */
function PlanGroup({ label, names }: { label: string; names: string[] }) {
  if (names.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="text-sm text-muted-foreground">{label}:</span>
      {names.map((n) => (
        <Badge key={n} variant="secondary" className="text-xs">
          {n}
        </Badge>
      ))}
    </div>
  );
}

// The backend's per-resource classification (w6/m125, w4/m133) in the order a
// reviewer reads it: what will be provisioned, what will change, what cannot
// apply, then what is already in place. Detach has its own alert below.
const OPERATION_ORDER = ["create", "update", "error", "noop"] as const;
type ListedOperation = (typeof OPERATION_ORDER)[number];

const OPERATION_LABEL: Record<ListedOperation, keyof typeof en> = {
  create: "blueprints.previewOpCreate",
  update: "blueprints.previewOpUpdate",
  error: "blueprints.previewOpError",
  noop: "blueprints.previewOpNoop",
};

const OPERATION_VARIANT: Record<
  ListedOperation,
  "default" | "secondary" | "destructive" | "outline"
> = {
  create: "default",
  update: "secondary",
  error: "destructive",
  noop: "outline",
};

const KIND_LABEL: Record<string, keyof typeof en> = {
  service: "blueprints.previewKindService",
  postgres: "blueprints.previewKindPostgres",
  key_value: "blueprints.previewKindKeyValue",
  env_var_group: "blueprints.previewKindEnvGroup",
};

/** One planned resource: its operation, name, kind, and what changes. */
function PlanActionRow({
  action,
  operation,
}: {
  action: BlueprintPlanAction;
  operation: ListedOperation;
}) {
  const { t } = useTranslations();
  const kind = KIND_LABEL[action.kind];
  const fields = (action.changedFields ?? [])
    .map((field) => field.path)
    .filter(Boolean);
  return (
    <li className="space-y-0.5">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant={OPERATION_VARIANT[operation]} className="text-xs">
          {t(OPERATION_LABEL[operation])}
        </Badge>
        <span className="text-sm font-medium">{action.name}</span>
        {kind && (
          <span className="text-xs text-muted-foreground">{t(kind)}</span>
        )}
      </div>
      {operation === "update" && fields.length > 0 && (
        <p className="break-words text-xs text-muted-foreground">
          {t("blueprints.previewChangedFields", { fields: fields.join(", ") })}
        </p>
      )}
      {operation === "update" && action.kind === "env_var_group" && (
        <p className="text-xs text-muted-foreground">
          {t("blueprints.previewEnvGroupReapplied")}
        </p>
      )}
      {operation === "error" && action.message && (
        <p className="break-words text-xs text-destructive">{action.message}</p>
      )}
    </li>
  );
}

/**
 * The parsed-plan summary shared by the create page's review step and the
 * detail page's pre-sync dialog (w8/m21): the per-resource plan, the count of
 * real changes, the detach alert, and the estimated-pricing panel. Page-specific
 * states (not-found, invalid, prompts) stay with their pages.
 *
 * The headline counts only actions that change something, so a sync that would
 * apply nothing says so instead of "1 resource to sync" (w4/m138). A plan
 * without per-action results (an older bex-api) falls back to the name groups.
 */
export function BlueprintPlanSummary({
  plan,
  pricing,
  note,
}: {
  plan: BlueprintPreviewPlan | null | undefined;
  pricing: BlueprintEstimatedPricing | null | undefined;
  note?: ReactNode;
}) {
  const { t } = useTranslations();
  const actions = plan?.actions ?? null;
  const detached = (actions ?? []).filter(
    (action) => action.operation === "detach",
  );
  const changes = actions
    ? actions.filter((action) => action.operation !== "noop").length
    : (plan?.totalActions ?? 0);
  const listed = OPERATION_ORDER.flatMap((operation) =>
    (actions ?? [])
      .filter((action) => action.operation === operation)
      .map((action) => ({ action, operation })),
  );
  return (
    <>
      <div className="space-y-3 rounded-md border p-4">
        <p className="text-sm font-medium">
          {changes === 0
            ? t("blueprints.previewNoChanges")
            : t("blueprints.previewValid", { count: changes })}
        </p>
        {actions ? (
          <ul className="space-y-2">
            {listed.map(({ action, operation }) => (
              <PlanActionRow
                key={`${action.kind}:${action.name}:${action.sourcePath}`}
                action={action}
                operation={operation}
              />
            ))}
          </ul>
        ) : (
          <>
            <PlanGroup
              label={t("blueprints.previewServices")}
              names={(plan?.services ?? []).filter(Boolean)}
            />
            <PlanGroup
              label={t("blueprints.previewDatabases")}
              names={(plan?.databases ?? []).filter(Boolean)}
            />
            <PlanGroup
              label={t("blueprints.previewKeyValue")}
              names={(plan?.keyValue ?? []).filter(Boolean)}
            />
            <PlanGroup
              label={t("blueprints.previewEnvGroups")}
              names={(plan?.envGroups ?? []).filter(Boolean)}
            />
          </>
        )}
        {note}
      </div>
      {detached.length > 0 && (
        <Alert>
          <AlertTitle>{t("blueprints.detachPreviewTitle")}</AlertTitle>
          <AlertDescription>
            <p>{t("blueprints.detachWarning")}</p>
            <p>{t("blueprints.detachEstimateNote")}</p>
            <ul className="mt-2 space-y-1">
              {detached.map((resource) => (
                <li
                  key={`${resource.kind}:${resource.resourceId ?? resource.name}`}
                >
                  <span className="font-medium">{resource.name}</span>
                  {resource.resourceId && (
                    <code className="ml-2 break-all text-xs">
                      {resource.resourceId}
                    </code>
                  )}
                </li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      )}
      <EstimatedPricingPanel pricing={pricing} />
    </>
  );
}
