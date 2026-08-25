import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";

interface BlueprintStatusBadgeProps {
  status: string;
}

export function BlueprintStatusBadge({ status }: BlueprintStatusBadgeProps) {
  const { t } = useTranslations();

  // This badge serves TWO backend vocabularies, and only the first was ever
  // mapped (w10/m11/t005):
  //
  //   Blueprint.Status     — created | paused | in_sync | syncing | error
  //   BlueprintSync.State  — created | running | success | error
  //
  // (lego/backend/internal/store/blueprints.go). An unmapped value fell through
  // to a "secondary" pill whose label is `blueprints.statusUnknown`, whose
  // message is literally "{status}" — so Sync History rendered plain lowercase
  // "success" beside a styled red "Error" for the same column.
  //
  // `active` is kept: it maps to no current backend constant, but other callers
  // may still pass it and dropping it would regress them to the raw fallback.
  type BadgeVariant = "default" | "secondary" | "destructive" | "outline";
  const variantMap: Record<string, BadgeVariant> = {
    active: "default",
    in_sync: "default",
    syncing: "secondary",
    error: "destructive",
    paused: "outline",
    // BlueprintSync.State
    created: "outline",
    running: "secondary",
    success: "default",
  };
  const labelMap: Record<string, string> = {
    active: t("blueprints.statusActive"),
    in_sync: t("blueprints.statusInSync"),
    syncing: t("blueprints.statusSyncing"),
    error: t("blueprints.statusError"),
    paused: t("blueprints.statusPaused"),
    created: t("blueprints.stateCreated"),
    running: t("blueprints.stateRunning"),
    success: t("blueprints.stateSuccess"),
  };

  const variant = variantMap[status] ?? "secondary";
  const label = labelMap[status] ?? t("blueprints.statusUnknown", { status });

  return <Badge variant={variant}>{label}</Badge>;
}
