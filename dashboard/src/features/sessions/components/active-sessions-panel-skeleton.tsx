import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { TableCell, TableRow } from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { PanelTableFrame } from "@/common/components/panel-states";
import { ReservedButton } from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { activeSessionsColumns } from "@/features/sessions/lib/active-sessions-columns";

/**
 * The sessions read's loading frame (w4/686). The current session is always
 * listed, first and with no revoke: its row is the frame's one placeholder.
 */
export function ActiveSessionsTableFrame() {
  const { t } = useTranslations();
  return (
    <PanelTableFrame columns={activeSessionsColumns(t)}>
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
  );
}

/**
 * ActiveSessionsPanel's frame before it mounts (w3/067): its static title and
 * description, the "Sign out other sessions" button's box, then the sessions
 * read's loading frame. Every other session is a data-dependent row.
 */
export function ActiveSessionsPanelSkeleton() {
  const { t } = useTranslations();
  return (
    <Card aria-hidden="true" data-testid="active-sessions-panel-skeleton">
      <CardHeader>
        <CardTitle>{t("activeSessions.title")}</CardTitle>
        <CardDescription>{t("activeSessions.description")}</CardDescription>
        <CardAction>
          <ReservedButton variant="outline" size="sm">
            {t("activeSessions.signOutOthers")}
          </ReservedButton>
        </CardAction>
      </CardHeader>
      <CardContent>
        <ActiveSessionsTableFrame />
      </CardContent>
    </Card>
  );
}
