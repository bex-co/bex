import { TableRow, TableCell } from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { RevokeIconButton } from "@/common/components/revoke-icon-button";
import { useTranslations } from "@/common/hooks/use-translations";
import { RelativeAge } from "@/common/components/relative-time";
import type { SessionView } from "@/features/sessions/types";

export interface SessionRowProps {
  session: SessionView;
  onRevoke: (id: string) => Promise<boolean>;
  /** True while this row's revoke is in flight — disables its own control. */
  revoking: boolean;
}

/** One Active Sessions row: device/location, last active, and revoke (never for the current session). */
export function SessionRow({ session, onRevoke, revoking }: SessionRowProps) {
  const { t } = useTranslations();

  return (
    <TableRow>
      <TableCell className="max-w-[16rem] truncate">
        {session.userAgent ?? t("activeSessions.unknownDevice")}
        {session.current ? (
          <Badge variant="secondary" className="ml-2">
            {t("activeSessions.current")}
          </Badge>
        ) : null}
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        {session.location ?? session.ipAddress ?? "—"}
      </TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        <RelativeAge value={session.authenticatedAt} />
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        {session.current ? null : (
          <RevokeIconButton
            label={t("activeSessions.revoke")}
            confirmTitle={t("activeSessions.revokeConfirmTitle")}
            confirmBody={t("activeSessions.revokeConfirmBody")}
            cancelLabel={t("activeSessions.revokeCancel")}
            confirmLabel={t("activeSessions.revoke")}
            onConfirm={() => void onRevoke(session.id)}
            pending={revoking}
          />
        )}
      </TableCell>
    </TableRow>
  );
}
