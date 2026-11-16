import { AlertTriangle, Loader2, ScrollText, ShieldOff } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Table, TableBody } from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import { PanelCenteredState } from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import type { UseAuditLogResult } from "@/features/audit/hooks/use-audit-log";
import { AuditEventRow } from "@/features/audit/components/audit-event-row";
import { AuditLogExport } from "@/features/audit/components/audit-log-export";
import {
  AuditLogPlanNotice,
  AuditLogTableFrame,
  AuditLogTableHeader,
} from "@/features/audit/components/audit-log-panel-skeleton";
import { useAuditPlanKnownMissing } from "@/features/audit/hooks/use-audit-plan-known-missing";

/**
 * Workspace Settings → Audit Logs (w4/m14; moved from account settings by
 * w3/m305 to Render's place): a workspace's write-verb trail, newest-first —
 * the human-facing counterpart of w4/m10's REST/GraphQL surface — with Render's
 * date-range CSV export. Its `state` (the `useAuditLog` result) is owned by the
 * caller. Entirely admin-gated (unlike Team, which shows a read-only list to
 * non-admins): a `forbidden` result renders nothing — a non-admin shouldn't
 * even learn this feature exists.
 */
export function AuditLogPanel({
  state,
  ownerId,
}: {
  state: UseAuditLogResult;
  /** The workspace the export reads; the caller resolved it. */
  ownerId: string;
}) {
  const { t } = useTranslations();
  const {
    events,
    loading,
    loadingMore,
    error,
    forbidden,
    unavailable,
    planRequired,
    hasMore,
    loadMore,
  } = state;

  // While the first page loads, draw the state it settles to (w4/684): the
  // plan notice for a workspace known to be below Pro, otherwise the export
  // row and a full first page's table frame.
  const belowPlan = useAuditPlanKnownMissing();
  const planNotice = planRequired || (loading && belowPlan);

  if (forbidden) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("audit.title")}</CardTitle>
        <CardDescription>{t("audit.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {!planNotice && !unavailable ? (
          <AuditLogExport ownerId={ownerId} />
        ) : null}
        {planNotice ? (
          <AuditLogPlanNotice />
        ) : unavailable ? (
          <PanelCenteredState
            icon={<ShieldOff />}
            title={t("audit.unavailableTitle")}
            body={t("audit.unavailableBody")}
          />
        ) : error ? (
          <PanelCenteredState
            icon={<AlertTriangle />}
            title={t("audit.errorTitle")}
            body={t("audit.errorBody")}
          />
        ) : loading ? (
          <AuditLogTableFrame />
        ) : events.length === 0 ? (
          <PanelCenteredState
            icon={<ScrollText />}
            title={t("audit.emptyTitle")}
            body={t("audit.emptyBody")}
          />
        ) : (
          <div className="space-y-4">
            <Table>
              <AuditLogTableHeader />
              <TableBody>
                {events.map((event) => (
                  <AuditEventRow key={event.id} event={event} />
                ))}
              </TableBody>
            </Table>
            {hasMore ? (
              <Button
                variant="outline"
                size="sm"
                disabled={loadingMore}
                onClick={() => loadMore()}
              >
                {loadingMore ? <Loader2 className="animate-spin" /> : null}
                {t("audit.loadMore")}
              </Button>
            ) : null}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
