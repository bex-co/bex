import { GitPullRequestDraft } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentSessionView } from "@/features/agent-sessions/types";

export interface DeliveryNoteCalloutProps {
  session: AgentSessionView;
}

/**
 * The inline note for a session whose agent finished without fully delivering
 * its work (ADR090 D5): the driver never commits for the agent, so the backend
 * records what is still uncommitted, unpushed or missing a pull request. The
 * work stays in the retained sandbox, so the next step is a follow-up turn.
 */
export function DeliveryNoteCallout({ session }: DeliveryNoteCalloutProps) {
  const { t } = useTranslations();
  if (!session.deliveryNote || session.phase === "failed") return null;
  return (
    <Alert>
      <GitPullRequestDraft />
      <AlertTitle>{t("agentSessions.deliveryNoteTitle")}</AlertTitle>
      <AlertDescription className="flex flex-col items-start gap-1">
        <span>{session.deliveryNote}</span>
        <span className="text-muted-foreground">
          {t("agentSessions.deliveryNoteHint")}
        </span>
      </AlertDescription>
    </Alert>
  );
}
