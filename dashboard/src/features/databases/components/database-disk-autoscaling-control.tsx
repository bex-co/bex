import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useUpdateDatabaseDiskAutoscaling } from "@/features/databases/hooks/use-update-database-disk-autoscaling";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import type { DatabaseDetailView } from "@/features/databases/types";

export interface DatabaseDiskAutoscalingControlProps {
  database: DatabaseDetailView;
  onChanged: () => void;
}

export function DatabaseDiskAutoscalingControl({
  database,
  onChanged,
}: DatabaseDiskAutoscalingControlProps) {
  const { t } = useTranslations();
  const { updateDiskAutoscaling, busy } = useUpdateDatabaseDiskAutoscaling();
  const { instanceTypes } = useDatabaseInstanceTypes();
  const plan = instanceTypes.find((it) => it.id === database.plan);
  const enableBlocked = !plan?.supportsDiskAutoscaling;
  const planBlocked = plan != null && enableBlocked;
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");

  async function handleChange(enabled: boolean) {
    if (createDenied) return;
    if (enabled && enableBlocked) return;
    if (await updateDiskAutoscaling(database.id, enabled)) onChanged();
  }

  return (
    // Render's Plan page puts Storage Autoscaling right under Storage
    // (w3/m641, docs/render-artifacts/postgres-dashboard.md), so this is a
    // row of the plan card under its Storage row, with the same anatomy.
    <div className="flex items-center justify-between gap-4 border-t pt-4">
      <div className="min-w-0 space-y-1">
        <label
          htmlFor="database-disk-autoscaling"
          className="block cursor-pointer text-sm font-medium"
        >
          {t("databases.diskAutoscalingLabel")}
        </label>
        <span className="block text-sm text-muted-foreground">
          {plan?.maxStorageGB != null
            ? t("databases.diskAutoscalingSize", {
                current: database.diskSizeGB ?? 0,
                cap: plan.maxStorageGB,
              })
            : t("databases.diskAutoscalingCurrentSize", {
                current: database.diskSizeGB ?? 0,
              })}
        </span>
        {planBlocked ? (
          <span
            id="database-disk-autoscaling-hint"
            className="block text-sm text-muted-foreground"
          >
            {t("databases.diskAutoscalingPlanUnsupported")}
          </span>
        ) : null}
      </div>
      <PermissionTooltip reason={createReason}>
        <Switch
          id="database-disk-autoscaling"
          checked={database.diskAutoscalingEnabled}
          disabled={
            busy ||
            createDenied ||
            (enableBlocked && !database.diskAutoscalingEnabled)
          }
          aria-describedby="database-disk-autoscaling-hint"
          onCheckedChange={(enabled) => void handleChange(enabled)}
        />
      </PermissionTooltip>
      {/* When blocked, the visible note above is the description; repeating
          it here made screen readers announce it twice. */}
      {planBlocked ? null : (
        <span id="database-disk-autoscaling-hint" className="sr-only">
          {t("databases.diskAutoscalingHint")}
        </span>
      )}
    </div>
  );
}
