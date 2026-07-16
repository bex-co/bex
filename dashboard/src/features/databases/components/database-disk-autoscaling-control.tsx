import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { useUpdateDatabaseDiskAutoscaling } from "@/features/databases/hooks/use-update-database-disk-autoscaling";
import type { DatabaseDetailView } from "@/features/databases/types";

// Build-tested against lego/types/tiers/tiers.yaml, the Go runtime source used
// by both the operator and backend MCP description.
export const DISK_AUTOSCALING_CAP_GB = 16 * 1024;

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

  async function handleChange(enabled: boolean) {
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
          {t("databases.diskAutoscalingSize", {
            current: database.diskSizeGB ?? 0,
            cap: DISK_AUTOSCALING_CAP_GB,
          })}
        </span>
      </div>
      <Switch
        id="database-disk-autoscaling"
        checked={database.diskAutoscalingEnabled}
        disabled={busy}
        aria-describedby="database-disk-autoscaling-hint"
        onCheckedChange={(enabled) => void handleChange(enabled)}
      />
      <span id="database-disk-autoscaling-hint" className="sr-only">
        {t("databases.diskAutoscalingHint")}
      </span>
    </div>
  );
}
