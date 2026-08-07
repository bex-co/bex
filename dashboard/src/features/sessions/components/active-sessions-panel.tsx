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
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
} from "@/common/components/ui/table";
import { Button } from "@/common/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/common/components/ui/alert-dialog";
import {
  PanelCenteredState,
  PanelTableSkeleton,
  TableActionsHead,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { useActiveSessions } from "@/features/sessions/hooks/use-active-sessions";
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

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("activeSessions.title")}</CardTitle>
        <CardDescription>{t("activeSessions.description")}</CardDescription>
        <CardAction>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button
                variant="outline"
                size="sm"
                disabled={!hasOtherSessions || signingOutOthers}
              >
                {signingOutOthers ? <Loader2 className="animate-spin" /> : null}
                {t("activeSessions.signOutOthers")}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  {t("activeSessions.signOutOthersConfirmTitle")}
                </AlertDialogTitle>
                <AlertDialogDescription>
                  {t("activeSessions.signOutOthersConfirmBody")}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>
                  {t("activeSessions.revokeCancel")}
                </AlertDialogCancel>
                <AlertDialogAction onClick={() => void signOutOthers()}>
                  {t("activeSessions.signOutOthers")}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
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
          <PanelTableSkeleton />
        ) : sessions.length === 0 ? (
          <PanelCenteredState
            icon={<Monitor />}
            title={t("activeSessions.emptyTitle")}
            body={t("activeSessions.emptyBody")}
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("activeSessions.colDevice")}</TableHead>
                <TableHead>{t("activeSessions.colLocation")}</TableHead>
                <TableHead>{t("activeSessions.colLastActive")}</TableHead>
                <TableActionsHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {sessions.map((session) => (
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
