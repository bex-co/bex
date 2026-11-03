import { useState } from "react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { BlueprintPlanSummary } from "@/features/blueprints/components/blueprint-plan-summary";
import {
  manualEnvVarKey,
  type PlannedSyncView,
} from "@/features/blueprints/hooks/use-planned-sync";

export interface PlannedSyncPanelProps {
  planned: PlannedSyncView | null;
  loading: boolean;
  busy: boolean;
  onApprove: (values: Record<string, string>) => Promise<boolean>;
  onReplan: () => Promise<void>;
}

/**
 * A created Blueprint's waiting plan (w2/m256 t012): what approving would do,
 * the sync:false values it needs, and the Approve control — the only way such
 * a Blueprint applies, as in Render's plan-then-approve flow.
 */
export function PlannedSyncPanel({
  planned,
  loading,
  busy,
  onApprove,
  onReplan,
}: PlannedSyncPanelProps) {
  const { t } = useTranslations();
  const [values, setValues] = useState<Record<string, string>>({});

  if (loading && !planned) {
    return (
      <Card aria-hidden="true" data-skeleton-region="planned-sync">
        <CardHeader>
          <Skeleton className="h-5 w-40" />
        </CardHeader>
        <CardContent className="space-y-4">
          <Skeleton className="h-4 w-full max-w-md" />
          <div className="space-y-2">
            <Skeleton className="h-5 w-64" />
            <Skeleton className="h-5 w-56" />
          </div>
          <div className="flex gap-2">
            <Skeleton className="h-8 w-24" />
            <Skeleton className="h-8 w-28" />
          </div>
        </CardContent>
      </Card>
    );
  }

  const missing = planned
    ? planned.requiredManualEnvVars.some(
        (v) => !(values[manualEnvVarKey(v)] ?? "").trim(),
      )
    : true;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">
          {t("blueprints.plannedTitle")}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {planned ? (
          <>
            <p className="text-sm text-muted-foreground">
              {t("blueprints.plannedBody", {
                commit: planned.commitId.slice(0, 7),
              })}
            </p>
            {planned.actions.length > 0 ? (
              <BlueprintPlanSummary
                plan={{
                  mode: null,
                  services: null,
                  databases: null,
                  keyValue: null,
                  envGroups: null,
                  syncFalseVars: null,
                  totalActions: planned.actions.length,
                  actions: planned.actions,
                }}
                pricing={null}
              />
            ) : (
              <p className="text-sm text-muted-foreground">
                {t("blueprints.plannedActionsUnavailable")}
              </p>
            )}
            {planned.requiredManualEnvVars.length > 0 ? (
              <div className="space-y-3">
                <p className="text-sm">
                  {t("blueprints.plannedManualEnvVars")}
                </p>
                {planned.requiredManualEnvVars.map((v) => {
                  const key = manualEnvVarKey(v);
                  const id = `manual-env-${key}`;
                  return (
                    <div key={key} className="space-y-1">
                      <Label htmlFor={id}>
                        {t("blueprints.plannedManualEnvVarLabel", {
                          service: v.serviceName,
                          key: v.key,
                        })}
                      </Label>
                      <Input
                        id={id}
                        type="password"
                        autoComplete="off"
                        required
                        value={values[key] ?? ""}
                        onChange={(e) =>
                          setValues((prev) => ({
                            ...prev,
                            [key]: e.target.value,
                          }))
                        }
                      />
                    </div>
                  );
                })}
              </div>
            ) : null}
            <div className="flex flex-wrap gap-2">
              <Button
                size="sm"
                disabled={busy || missing}
                onClick={() => void onApprove(values)}
              >
                {t("blueprints.plannedApprove")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={busy}
                onClick={() => void onReplan()}
              >
                {t("blueprints.plannedReplan")}
              </Button>
            </div>
          </>
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              {t("blueprints.plannedNone")}
            </p>
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => void onReplan()}
            >
              {t("blueprints.plannedReplan")}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
