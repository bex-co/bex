import { Bot } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The agent-sessions list empty state: shown once the query resolves with no
 * sessions in the workspace. The composer above it is the call to action, so
 * this stays a quiet explanatory panel rather than repeating a create button.
 */
export function AgentSessionsEmptyState() {
  const { t } = useTranslations();
  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed p-10 text-center">
      <Bot className="size-8 text-muted-foreground" />
      <div>
        <p className="font-medium">{t("agentSessions.emptyTitle")}</p>
        <p className="mx-auto mt-1 max-w-md text-sm text-muted-foreground">
          {t("agentSessions.emptyBody")}
        </p>
      </div>
    </div>
  );
}
