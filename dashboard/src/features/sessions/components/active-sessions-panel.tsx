import { AlertTriangle, Loader2, Monitor } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import {
  PanelCenteredState,
  PanelTableHeader,
  PanelTableFrame,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { useActiveSessions } from "@/features/sessions/hooks/use-active-sessions";
import { orderSessions } from "@/features/sessions/lib/order-sessions";
import { SessionRow } from "@/features/sessions/components/session-row";

/**
 * Settings → Security & Compliance "Active sessions" card (w4/m18, folded from
 * inbox w4/006): every browser Kratos currently recognizes as this human, plus
 * a one-click "sign out other sessions" — the Kratos-owned counterpart of the
 * OAuth-scoped Connected Agents card. Account-scoped, no workspace wiring.
 */
export function ActiveSessionsPanel() {
  const { t } = useTranslations();
  const {
    sessions,
    loading,
    error,
    revoke,
    revoking,
    signOutOthers,
    signingOutOthers,
  } = useActiveSessions();

  const hasOtherSessions = sessions.some((s) => !s.current);
  const columns = [
    t("activeSessions.colDevice"),
    t("activeSessions.colLocation"),
    t("activeSessions.colSignedIn"),
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("activeSessions.title")}</CardTitle>
        <CardDescription>{t("activeSessions.description")}</CardDescription>
        <CardAction>
          <ConfirmDialog
            trigger={
              <Button
                variant="outline"
                size="sm"
                disabled={!hasOtherSessions || signingOutOthers}
              >
                {signingOutOthers ? <Loader2 className="animate-spin" /> : null}
                {t("activeSessions.signOutOthers")}
              </Button>
            }
            title={t("activeSessions.signOutOthersConfirmTitle")}
            description={t("activeSessions.signOutOthersConfirmBody")}
            cancelLabel={t("activeSessions.revokeCancel")}
            confirmLabel={t("activeSessions.signOutOthers")}
            onConfirm={() => void signOutOthers()}
          />
        </CardAction>
      </CardHeader>
      <CardContent>
        {error ? (
          <PanelCenteredState
            icon={<AlertTriangle />}
            title={t("activeSessions.errorTitle")}
            body={t("activeSessions.errorBody")}
          />
        ) : loading ? (
          // The current session is always listed, first and with no revoke
          // (w4/686): its row is the frame's one placeholder.
          <PanelTableFrame columns={columns}>
            <TableRow data-testid="current-session-placeholder">
              <TableCell className="max-w-[16rem] truncate">
                <Skeleton className="inline-block h-4 w-40 align-middle" />
                <Badge variant="secondary" className="ml-2">
                  {t("activeSessions.current")}
                </Badge>
              </TableCell>
              <TableCell>
                <Skeleton className="h-5 w-24" />
              </TableCell>
              <TableCell>
                <Skeleton className="h-5 w-20" />
              </TableCell>
              <TableCell />
            </TableRow>
          </PanelTableFrame>
        ) : sessions.length === 0 ? (
          <PanelCenteredState
            icon={<Monitor />}
            title={t("activeSessions.emptyTitle")}
            body={t("activeSessions.emptyBody")}
          />
        ) : (
          <Table>
            <PanelTableHeader columns={columns} />
            <TableBody>
              {orderSessions(sessions).map((session) => (
                <SessionRow
                  key={session.id}
                  session={session}
                  onRevoke={revoke}
                  revoking={revoking === session.id}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
