import { AlertTriangle, Loader2, ScrollText, ShieldOff } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
} from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import {
  PanelCenteredState,
  PanelTableSkeleton,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCurrentWorkspace } from "@/features/team/hooks/use-current-workspace";
import { useAuditLog } from "@/features/audit/hooks/use-audit-log";
import { AuditEventRow } from "@/features/audit/components/audit-event-row";

type StateKind = "unavailable" | "error";

/**
 * Settings → Audit Log (w4/m14): a workspace's write-verb trail, newest-first —
 * the human-facing counterpart of w4/m10's REST/GraphQL surface. Entirely
 * admin-gated (unlike Team, which shows a read-only list to non-admins): a
 * `forbidden` result renders nothing, not an empty/error card, since a
 * non-admin shouldn't even learn this feature exists.
 */
export function AuditLogPanel() {
  const { t } = useTranslations();
  const { workspace } = useCurrentWorkspace();
  const {
    events,
    loading,
    loadingMore,
    error,
    forbidden,
    unavailable,
    hasMore,
    loadMore,
  } = useAuditLog(workspace?.id ?? null);

  if (forbidden) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("audit.title")}</CardTitle>
        <CardDescription>{t("audit.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {unavailable ? (
          <StatePanel kind="unavailable" />
        ) : error ? (
          <StatePanel kind="error" />
        ) : loading ? (
          <PanelTableSkeleton />
        ) : events.length === 0 ? (
          <EmptyState />
        ) : (
          <div className="space-y-4">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("audit.columnTimestamp")}</TableHead>
                  <TableHead>{t("audit.columnActor")}</TableHead>
                  <TableHead>{t("audit.columnAction")}</TableHead>
                  <TableHead>{t("audit.columnStatus")}</TableHead>
                  <TableHead>{t("audit.columnResource")}</TableHead>
                </TableRow>
              </TableHeader>
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

function EmptyState() {
  const { t } = useTranslations();
  return (
    <PanelCenteredState
      icon={<ScrollText />}
      title={t("audit.emptyTitle")}
      body={t("audit.emptyBody")}
    />
  );
}

function StatePanel({ kind }: { kind: StateKind }) {
  const { t } = useTranslations();
  const copy = {
    unavailable: {
      icon: <ShieldOff />,
      title: t("audit.unavailableTitle"),
      body: t("audit.unavailableBody"),
    },
    error: {
      icon: <AlertTriangle />,
      title: t("audit.errorTitle"),
      body: t("audit.errorBody"),
    },
  }[kind];
  return (
    <PanelCenteredState icon={copy.icon} title={copy.title} body={copy.body} />
  );
}
