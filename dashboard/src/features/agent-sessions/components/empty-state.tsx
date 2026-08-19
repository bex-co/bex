import { Bot } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The agent-sessions list empty state: shown once the query resolves with no
 * sessions in the workspace. The composer above it is the call to action, so
 * this stays a quiet explanatory panel rather than repeating a create button.
 */
export function AgentSessionsEmptyState({
  mode = "default",
  onClearFilters,
}: {
  mode?: "default" | "archived" | "filtered";
  onClearFilters?: () => void;
}) {
  const { t } = useTranslations();
  const titleKey =
    mode === "archived"
      ? "agentSessions.emptyArchivedTitle"
      : mode === "filtered"
        ? "agentSessions.emptyFilteredTitle"
        : "agentSessions.emptyTitle";
  const bodyKey =
    mode === "archived"
      ? "agentSessions.emptyArchivedBody"
      : mode === "filtered"
        ? "agentSessions.emptyFilteredBody"
        : "agentSessions.emptyBody";
  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed p-10 text-center">
      <Bot className="size-8 text-muted-foreground" />
      <div>
        <p className="font-medium">{t(titleKey)}</p>
        <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">
          {t(bodyKey)}
        </p>
      </div>
      {mode !== "default" && onClearFilters ? (
        <Button size="sm" variant="outline" onClick={onClearFilters}>
          {t("agentSessions.clearFilters")}
        </Button>
      ) : null}
    </div>
  );
}
