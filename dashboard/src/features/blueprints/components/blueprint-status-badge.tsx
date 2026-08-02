import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";

interface BlueprintStatusBadgeProps {
  status: string;
}

export function BlueprintStatusBadge({ status }: BlueprintStatusBadgeProps) {
  const { t } = useTranslations();

  type BadgeVariant = "default" | "secondary" | "destructive" | "outline";
  const variantMap: Record<string, BadgeVariant> = {
    active: "default",
    in_sync: "default",
    syncing: "secondary",
    error: "destructive",
    paused: "outline",
  };
  const labelMap: Record<string, string> = {
    active: t("blueprints.statusActive"),
    in_sync: t("blueprints.statusInSync"),
    syncing: t("blueprints.statusSyncing"),
    error: t("blueprints.statusError"),
    paused: t("blueprints.statusPaused"),
  };

  const variant = variantMap[status] ?? "secondary";
  const label = labelMap[status] ?? t("blueprints.statusUnknown", { status });

  return <Badge variant={variant}>{label}</Badge>;
}
