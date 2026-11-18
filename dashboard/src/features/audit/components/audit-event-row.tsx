import { TableRow, TableCell } from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspaceSubjectLabel } from "@/common/hooks/use-workspace-subject-label";
import { LocalDateTime } from "@/common/components/local-time";
import type { AuditEvent } from "@/features/audit/types";
import { shortUserAgent } from "@/features/audit/lib/short-user-agent";
import { deletedUserLabel } from "@/features/audit/lib/audit-csv";

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
  // Named as the CSV export names them (w9/101), never by the stored subject:
  // a deleted user's is an internal marker, and REST's actor.id (the usr- id)
  // is what the roster and the export know a person by. The row's own email
  // still wins from first paint, after they left the roster (w9/m450).
  const subject = event.actorId || event.actor;
  const actor = useWorkspaceSubjectLabel(
    subject,
    event.actorEmail ||
      (event.actorDeleted && event.actorId
        ? deletedUserLabel(event.actorId)
        : null),
  );
  const client = shortUserAgent(event.userAgent);
  // What a rename, a custom-domain verb, an env-group link (w5/183), an env
  // var row (w9/m449) or an OAuth access grant (w4/m454) changed, or the
  // instance a shell session ran on (w9/m451).
  const change = event.displayNameTo
    ? t("audit.changeRename", {
        from: event.displayNameFrom,
        to: event.displayNameTo,
      })
    : event.domain
      ? t("audit.changeDomain", { domain: event.domain })
      : event.envGroupName
        ? t("audit.changeEnvGroup", { name: event.envGroupName })
        : event.envVarKeys.length > 0
          ? t("audit.changeEnvVars", { keys: event.envVarKeys.join(", ") })
          : event.oauthGrantScopes.length > 0
            ? t("audit.changeGrantScopes", {
                scopes: event.oauthGrantScopes.join(" "),
              })
            : event.instance
              ? t("audit.changeInstance", { instance: event.instance })
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
        {/* Inline, so the first line stays the cell's own (w4/684); the
            title carries the whole label the cell may truncate. */}
        {subject ? <span title={actor}>{actor}</span> : t("audit.actorUnknown")}
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
