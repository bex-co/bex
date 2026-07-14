import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";

interface BlueprintStatusBadgeProps {
  status: string;
}

export function BlueprintStatusBadge({ status }: BlueprintStatusBadgeProps) {
  const { t } = useTranslations();

  const variant =
    status === "active"
      ? ("default" as const)
      : ("secondary" as const);

  const label =
    status === "active"
      ? t("blueprints.statusActive")
      : t("blueprints.statusUnknown", { status });

  return <Badge variant={variant}>{label}</Badge>;
}
