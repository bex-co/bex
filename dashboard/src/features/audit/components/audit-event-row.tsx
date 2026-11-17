import { TableRow, TableCell } from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspaceSubjectLabel } from "@/common/hooks/use-workspace-subject-label";
import { LocalDateTime } from "@/common/components/local-time";
import type { AuditEvent } from "@/features/audit/types";
import { shortUserAgent } from "@/features/audit/lib/short-user-agent";

export interface AuditEventRowProps {
  event: AuditEvent;
}

/** One audit-trail row: when, who (+ how they authenticated), from where (the
 *  client address and user agent, w9/m450), what, whether it was allowed, and
 *  what it acted on — the row-level counterpart of
 *  features/team/components/member-row.tsx. Every cell is at most two lines
 *  (a text-sm line and one truncated text-xs line), the height
 *  AuditLogTableFrame reserves per placeholder row (w4/684). */
export function AuditEventRow({ event }: AuditEventRowProps) {
  const { t } = useTranslations();
  // The person's email from the row itself names them at first paint, and
  // still does after they left the workspace's member list (w9/m450).
  const actor = useWorkspaceSubjectLabel(event.actor ?? "", event.actorEmail);
  const client = shortUserAgent(event.userAgent);
  // What a rename, a custom-domain verb or an env-group link changed (w5/183).
  const change = event.displayNameTo
    ? t("audit.changeRename", {
        from: event.displayNameFrom,
        to: event.displayNameTo,
      })
    : event.domain
      ? t("audit.changeDomain", { domain: event.domain })
      : event.envGroupName
        ? t("audit.changeEnvGroup", { name: event.envGroupName })
        : null;
  // How the actor authenticated, and for a third-party OAuth grant which
  // client and scopes, share one truncated second line (w4/684): every row
  // stays the two-line height the loading frame reserves.
  const actorDetail = [
    event.actorMethod,
    event.oauthClientId
      ? t("audit.oauthDelegation", {
          client: event.oauthClientId,
          scopes: event.oauthScopes.join(" ") || "—",
        })
      : "",
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <TableRow>
      {/* Full date + time (an audit trail spans days, unlike the logs
          viewer's time-only clock) — a missing/unparseable instant renders
          blank, never "Invalid Date". */}
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        <LocalDateTime value={event.timestamp} fallback={null} />
      </TableCell>
      <TableCell className="max-w-[16rem] truncate text-sm">
        {event.actor ? actor : t("audit.actorUnknown")}
        {actorDetail ? (
          <span
            className="text-muted-foreground block truncate text-xs"
            title={actorDetail}
          >
            {actorDetail}
          </span>
        ) : null}
      </TableCell>
      {/* Where the actor acted from: the address, and the client in short
          with the full user agent on hover. A row no request of the actor
          caused (the system's own) has neither. */}
      <TableCell className="text-muted-foreground max-w-[12rem] truncate text-sm">
        {event.sourceIp || event.userAgent ? (
          <>
            <span className="font-mono">{event.sourceIp || "—"}</span>
            {client ? (
              <span className="block truncate text-xs" title={event.userAgent}>
                {client}
              </span>
            ) : null}
          </>
        ) : (
          "—"
        )}
      </TableCell>
      {/* nowrap so a dotted action id (e.g. `apikeys.ListAPIKeys`) never gets
          letter-wrapped when the auto-layout table is squeezed on a narrow
          viewport — that collapse blew rows up to hundreds of px tall. The
          table's `overflow-x-auto` container scrolls horizontally instead. */}
      <TableCell className="text-sm whitespace-nowrap">
        {event.action}
      </TableCell>
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
            <span
              className="block truncate text-xs"
              title={change ? `${event.resource} · ${change}` : event.resource}
            >
              <span className="font-mono">{event.resource}</span>
              {change ? ` · ${change}` : null}
            </span>
          </>
        ) : (
          <>
            <span className="font-mono">{event.resource}</span>
            {change ? (
              <span className="block truncate text-xs" title={change}>
                {change}
              </span>
            ) : null}
          </>
        )}
      </TableCell>
    </TableRow>
  );
}
