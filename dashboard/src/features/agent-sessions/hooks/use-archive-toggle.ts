import { useCallback, useState } from "react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import { agentSessionErrorMessage } from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/**
 * The one archive/unarchive toggle behavior (ADR065 D1) shared by the list's
 * per-row action and the detail header's menu item: flip on `isArchived`,
 * toast the outcome (typed error copy via `agentSessionErrorMessage`), and
 * refresh through `onChanged`. `busyId` is the session currently in flight, so
 * a list renders one hook instance and passes plain props to its rows.
 */
export function useArchiveToggle(onChanged?: () => void | Promise<unknown>) {
  const { t } = useTranslations();
  const { archive, unarchive } = useAgentSessionMutations();
  const [busyId, setBusyId] = useState<string | null>(null);

  const undoArchive = useCallback(
    async (id: string) => {
      try {
        await unarchive(id);
        toast.success(t("agentSessions.unarchiveSuccess"));
      } catch (err) {
        toast.error(agentSessionErrorMessage(err, t));
      }
    },
    [t, unarchive],
  );

  const toggle = useCallback(
    async (session: Pick<AgentSessionView, "id" | "isArchived">) => {
      setBusyId(session.id);
      try {
        if (session.isArchived) {
          await unarchive(session.id);
          toast.success(t("agentSessions.unarchiveSuccess"));
        } else {
          await archive(session.id);
          toast.success(t("agentSessions.archiveSuccess"), {
            action: {
              label: t("agentSessions.undoArchive"),
              onClick: () => void undoArchive(session.id),
            },
          });
        }
        await onChanged?.();
      } catch (err) {
        toast.error(agentSessionErrorMessage(err, t));
      } finally {
        setBusyId(null);
      }
    },
    [archive, unarchive, onChanged, t, undoArchive],
  );

  return { toggle, busyId };
}
