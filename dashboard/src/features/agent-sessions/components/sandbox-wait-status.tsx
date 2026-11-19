import { useNow } from "@/common/hooks/use-now";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  agentSessionWaitMs,
  formatDurationShort,
} from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/**
 * "Waiting for a sandbox machine · 3m 12s" (ADR047 § w10/m497): a session whose
 * turn waits because the sandbox pool has no room, with the time it has waited,
 * as Devin's "waiting for machine capacity" notice. It ticks on its own clock,
 * which every waiting session on a page shares. The time is a `timer`, a live
 * region that is not announced, so a status region around the label speaks
 * once, not every second. Renders nothing for a session that does not wait.
 */
export function SandboxWaitStatus({
  session,
}: {
  session: Pick<AgentSessionView, "waitingFor" | "waitingSince">;
}) {
  const { t } = useTranslations();
  const now = useNow(1_000);
  if (!session.waitingFor) return null;
  const waited = agentSessionWaitMs(session, now);
  return (
    <span data-testid="sandbox-wait" data-waiting-for={session.waitingFor}>
      {t("agentSessions.statusPhrase.waitingForSandbox")}
      {waited != null ? (
        // The time reads the viewer's clock, which the SSR pass and hydration
        // sample at different moments (the RelativeAge convention).
        <span role="timer" suppressHydrationWarning>
          {` · ${formatDurationShort(waited)}`}
        </span>
      ) : null}
    </span>
  );
}
