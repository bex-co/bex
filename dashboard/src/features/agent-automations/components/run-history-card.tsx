import { Link } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { LocalDateTime } from "@/common/components/local-time";
import { useTranslations } from "@/common/hooks/use-translations";
import { AutomationLoadError } from "@/features/agent-automations/components/automation-load-state";
import { OutcomeBadge } from "@/features/agent-automations/components/outcome-badge";
import type { UseAutomationRunsResult } from "@/features/agent-automations/hooks/use-agent-automation-runs";
import {
  runSessionStatus,
  type RunSessionStatus,
} from "@/features/agent-automations/lib/display";

const SKELETON_ROWS = 3;

/**
 * The run history (ADR101 D3, D7): fire time, outcome, the session it started,
 * ticks skipped while the scheduler was down, and the error. A started run
 * whose session waits for a sandbox machine, or whose sandbox did not start,
 * says so in the outcome, with the start's cause as its error (w10/m497).
 * `runs` null is the pending state: the same card and table header, bars for
 * the rows.
 */
export function RunHistoryCard({
  runs,
}: {
  runs: UseAutomationRunsResult | null;
}) {
  const { t } = useTranslations();
  return (
    <Card data-testid="run-history">
      <CardHeader>
        <CardTitle>{t("agentAutomations.runsTitle")}</CardTitle>
        <CardDescription>
          {t("agentAutomations.runsDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {runs?.error && runs.runs.length === 0 ? (
          <AutomationLoadError
            error={runs.error}
            onRetry={() => void runs.refetch()}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("agentAutomations.runs.fireAt")}</TableHead>
                <TableHead>{t("agentAutomations.runs.outcome")}</TableHead>
                <TableHead>{t("agentAutomations.runs.session")}</TableHead>
                <TableHead className="text-right">
                  {t("agentAutomations.runs.missed")}
                </TableHead>
                <TableHead>{t("agentAutomations.runs.error")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {runs == null || runs.loading ? (
                Array.from({ length: SKELETON_ROWS }, (_, row) => (
                  <TableRow key={row} data-skeleton-region="run-row">
                    <TableCell>
                      <Skeleton className="h-5 w-32" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-[1.375rem] w-20" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-5 w-20" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="ml-auto h-5 w-6" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-5 w-40" />
                    </TableCell>
                  </TableRow>
                ))
              ) : runs.runs.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className="text-muted-foreground h-14 text-center"
                  >
                    {t("agentAutomations.runsEmpty")}
                  </TableCell>
                </TableRow>
              ) : (
                runs.runs.map((run) => {
                  const status = runSessionStatus(run);
                  return (
                    <TableRow key={run.fireAt} data-outcome={run.outcome}>
                      <TableCell className="whitespace-nowrap">
                        <LocalDateTime value={run.fireAt} />
                      </TableCell>
                      <TableCell>
                        <OutcomeBadge outcome={run.outcome} status={status} />
                      </TableCell>
                      <TableCell>
                        {run.sessionId ? (
                          <Link
                            to="/agents/$agentSessionId"
                            params={{ agentSessionId: run.sessionId }}
                            className="underline underline-offset-2"
                          >
                            {t("agentAutomations.runs.openSession")}
                          </Link>
                        ) : (
                          "—"
                        )}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {run.missedTicks > 0 ? run.missedTicks : "—"}
                      </TableCell>
                      <TableCell className="max-w-xs break-words whitespace-normal">
                        <RunError error={run.error} status={status} />
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        )}
        {runs?.hasMore ? (
          <div className="flex justify-center">
            <Button
              variant="outline"
              size="sm"
              disabled={runs.loadingMore}
              onClick={() => void runs.loadMore()}
            >
              {runs.loadingMore ? <Loader2 className="animate-spin" /> : null}
              {t("agentAutomations.loadMore")}
            </Button>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

/**
 * A run's error, or, for a started run whose session's sandbox did not start,
 * why (w10/m497): the run itself did not fail, so it carries no error of its
 * own.
 */
function RunError({
  error,
  status,
}: {
  error?: string | null;
  status: RunSessionStatus | null;
}) {
  const { t } = useTranslations();
  if (error) return <span className="text-destructive">{error}</span>;
  if (status?.kind !== "did_not_start") return "—";
  return (
    <span
      className="text-destructive"
      data-testid="run-start-cause"
      data-cause={status.cause}
    >
      {t(`agentSessions.failureCause.${status.cause}`)}
    </span>
  );
}
