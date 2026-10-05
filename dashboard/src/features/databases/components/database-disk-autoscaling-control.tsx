import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
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

  async function handleChange(enabled: boolean) {
    if (enabled && enableBlocked) return;
    if (await updateDiskAutoscaling(database.id, enabled)) onChanged();
  }

  return (
    <div className="flex items-center gap-3 rounded-md border bg-muted/30 px-3 py-2">
      <div className="min-w-0 text-right">
        <label
          htmlFor="database-disk-autoscaling"
          className="block cursor-pointer text-xs font-medium"
        >
          {t("databases.diskAutoscalingLabel")}
        </label>
        <span className="block text-xs text-muted-foreground">
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
            className="block text-xs text-muted-foreground"
          >
            {t("databases.diskAutoscalingPlanUnsupported")}
          </span>
        ) : null}
      </div>
      <Switch
        id="database-disk-autoscaling"
        checked={database.diskAutoscalingEnabled}
        disabled={busy || (enableBlocked && !database.diskAutoscalingEnabled)}
        aria-describedby="database-disk-autoscaling-hint"
        onCheckedChange={(enabled) => void handleChange(enabled)}
      />
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
