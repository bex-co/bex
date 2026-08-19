import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Empty copy under the composer. The default view is a quiet line — the
 * prompt box is the CTA. Archived / filtered empties stay distinct.
 */
export function AgentSessionsEmptyState({
  mode = "default",
  onClearFilters,
}: {
  mode?: "default" | "archived" | "filtered";
  onClearFilters?: () => void;
}) {
  const { t } = useTranslations();
  if (mode === "default") {
    return (
      <p className="text-muted-foreground px-1 text-sm">
        {t("agentSessions.emptyBody")}
      </p>
    );
  }
  const titleKey =
    mode === "archived"
      ? "agentSessions.emptyArchivedTitle"
      : "agentSessions.emptyFilteredTitle";
  const bodyKey =
    mode === "archived"
      ? "agentSessions.emptyArchivedBody"
      : "agentSessions.emptyFilteredBody";
  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed p-10 text-center">
      <div>
        <p className="font-medium">{t(titleKey)}</p>
        <p className="text-muted-foreground mx-auto mt-1 max-w-md text-sm">
          {t(bodyKey)}
        </p>
      </div>
      {onClearFilters ? (
        <Button size="sm" variant="outline" onClick={onClearFilters}>
          {t("agentSessions.clearFilters")}
        </Button>
      ) : null}
    </div>
  );
}
