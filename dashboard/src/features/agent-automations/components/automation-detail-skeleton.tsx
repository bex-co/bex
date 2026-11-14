import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { AutomationActionBar } from "@/features/agent-automations/components/automation-action-bar";
import { AutomationSubPageFrame } from "@/features/agent-automations/components/automation-frame";
import { AutomationSettingsCard } from "@/features/agent-automations/components/automation-settings-card";
import { RunHistoryCard } from "@/features/agent-automations/components/run-history-card";
import { NO_ACTIONS } from "@/features/agent-automations/lib/display";

/**
 * The task card's body: a bar standing in for the task text. Shared with the
 * ready card so its header and bounds are the same.
 */
export function AutomationTaskCard({ task }: { task: string | null }) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("agentAutomations.taskTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {task == null ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-4/5" />
            <Skeleton className="h-4 w-3/5" />
          </div>
        ) : (
          <p className="line-clamp-10 text-sm break-words whitespace-pre-wrap">
            {task}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

/**
 * /automations/<id> while it loads: the ready page's own frame, action bar
 * (every action present and disabled), settings card, task card and run table,
 * with bars where the automation's data goes.
 */
export function AutomationDetailSkeleton() {
  const { t } = useTranslations();
  return (
    <AutomationSubPageFrame
      title={
        <>
          <Skeleton className="h-6 w-48 max-w-[55vw]" />
          <span className="sr-only">{t("common.loading")}</span>
        </>
      }
      backLabel={t("agentAutomations.backToList")}
      width="detail"
    >
      <div
        aria-busy="true"
        data-skeleton-frame="true"
        data-route-skeleton="automation-detail"
        className="space-y-6"
      >
        <AutomationActionBar
          automation={null}
          actions={NO_ACTIONS}
          busy={false}
          onRunNow={() => {}}
          onToggle={() => {}}
          onTakeOver={async () => false}
          onDelete={async () => false}
        />
        <div data-skeleton-region="settings">
          <AutomationSettingsCard automation={null} />
        </div>
        <div data-skeleton-region="task">
          <AutomationTaskCard task={null} />
        </div>
        <div data-skeleton-region="run-history">
          <RunHistoryCard runs={null} />
        </div>
      </div>
    </AutomationSubPageFrame>
  );
}
