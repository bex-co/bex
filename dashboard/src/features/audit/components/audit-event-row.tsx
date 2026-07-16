import { TableRow, TableCell } from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AuditEvent } from "@/features/audit/types";

// Full date + time (an audit trail spans days, unlike the logs viewer's
// time-only clock, features/logs/lib/format.ts's formatLogTimestamp) — local
// time, blank rather than "Invalid Date" for an unparseable/missing stamp.
function formatAuditTimestamp(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString();
}

export interface AuditEventRowProps {
  event: AuditEvent;
}

/** One audit-trail row: when, who (+ how they authenticated), what, whether it
 *  was allowed, and what it acted on — the row-level counterpart of
 *  features/team/components/member-row.tsx. */
export function AuditEventRow({ event }: AuditEventRowProps) {
  const { t } = useTranslations();

  return (
    <TableRow>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        {formatAuditTimestamp(event.timestamp)}
      </TableCell>
      <TableCell className="max-w-[16rem] truncate font-mono text-sm">
        {event.actor || t("audit.actorUnknown")}
        {event.actorMethod ? (
          <span className="text-muted-foreground block text-xs">
            {event.actorMethod}
          </span>
        ) : null}
      </TableCell>
      <TableCell className="text-sm">{event.action}</TableCell>
      <TableCell>
        {event.status === "denied" ? (
          <Badge variant="destructive">{t("audit.statusDenied")}</Badge>
        ) : (
          <Badge variant="success">{t("audit.statusAllowed")}</Badge>
        )}
      </TableCell>
      <TableCell className="text-muted-foreground max-w-[16rem] truncate text-sm">
        {event.targetName ? (
          <>
            {event.targetName}
            <span className="block truncate font-mono text-xs">
              {event.resource}
            </span>
          </>
        ) : (
          <span className="font-mono">{event.resource}</span>
        )}
      </TableCell>
    </TableRow>
  );
}
