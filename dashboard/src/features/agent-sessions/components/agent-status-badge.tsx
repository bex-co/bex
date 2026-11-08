import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  agentStatusGroup,
  type AgentStatusGroup,
} from "@/features/agent-sessions/lib/lineage";
import type { AgentSessionView } from "@/features/agent-sessions/types";

const VARIANT: Record<
  AgentStatusGroup,
  "secondary" | "success" | "destructive" | "outline"
> = {
  working: "secondary",
  done: "success",
  failed: "destructive",
  canceled: "outline",
};

/** The status label of a spawned session, derived from its `state`. */
export function AgentStatusBadge({
  session,
}: {
  session: Pick<AgentSessionView, "state" | "phase">;
}) {
  const { t } = useTranslations();
  const group = agentStatusGroup(session);
  return (
    <Badge variant={VARIANT[group]} data-status={group}>
      {t(`agentSessions.lineage.status.${group}`)}
    </Badge>
  );
}
