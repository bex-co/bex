import { useNavigate } from "@tanstack/react-router";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { AutomationActionBar } from "@/features/agent-automations/components/automation-action-bar";
import {
  AutomationDetailSkeleton,
  AutomationTaskCard,
} from "@/features/agent-automations/components/automation-detail-skeleton";
import { AutomationSubPageFrame } from "@/features/agent-automations/components/automation-frame";
import { AutomationLoadError } from "@/features/agent-automations/components/automation-load-state";
import { AutomationSettingsCard } from "@/features/agent-automations/components/automation-settings-card";
import { RunHistoryCard } from "@/features/agent-automations/components/run-history-card";
import { useAgentAutomation } from "@/features/agent-automations/hooks/use-agent-automation";
import { useAutomationRuns } from "@/features/agent-automations/hooks/use-agent-automation-runs";
import { useAutomationOperations } from "@/features/agent-automations/hooks/use-automation-operations";
import { useAutomationViewer } from "@/features/agent-automations/hooks/use-automation-viewer";
import { pausedExplainKey } from "@/features/agent-automations/lib/display";

/**
 * /automations/<id> (ADR101 D9): the automation's settings, the actions its
 * viewer may take, and its run history.
 */
export function AutomationDetailPage({
  automationId,
}: {
  automationId: string;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { automation, loading, error, refetch } =
    useAgentAutomation(automationId);
  const runs = useAutomationRuns(automationId);
  const viewer = useAutomationViewer();
  const ops = useAutomationOperations();

  if (loading) return <AutomationDetailSkeleton />;
  if (!automation) {
    return (
      <AutomationSubPageFrame
        title={t("agentAutomations.pageTitle")}
        backLabel={t("agentAutomations.backToList")}
        width="detail"
      >
        <AutomationLoadError error={error} onRetry={() => void refetch()} />
      </AutomationSubPageFrame>
    );
  }

  const actions = viewer.actionsFor(automation.createdBy);
  const reason = pausedExplainKey(automation.pausedReason);
  return (
    <AutomationSubPageFrame
      title={automation.name}
      backLabel={t("agentAutomations.backToList")}
      width="detail"
    >
      {reason ? (
        <Alert role="status" data-testid="automation-paused-alert">
          <AlertDescription>
            {t(reason, { failures: automation.consecutiveFailures })}
          </AlertDescription>
        </Alert>
      ) : null}
      <AutomationActionBar
        automation={automation}
        actions={actions}
        busy={ops.busy}
        onRunNow={() => void ops.runNow(automation)}
        onToggle={() => void ops.toggle(automation)}
        onTakeOver={() => ops.takeOver(automation)}
        onDelete={async () => {
          const ok = await ops.remove(automation);
          if (ok) await navigate({ to: "/automations" });
          return ok;
        }}
      />
      <AutomationSettingsCard automation={automation} />
      <AutomationTaskCard task={automation.template.task} />
      <RunHistoryCard runs={runs} />
    </AutomationSubPageFrame>
  );
}
