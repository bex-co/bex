import type { AuditLogsQuery } from "@/graphql/definitions";
import type { AuditEvent } from "@/features/audit/types";

/** The first page, and each Load more page, of the audit table. */
export const AUDIT_PAGE_SIZE = 20;

type RawEvent = NonNullable<AuditLogsQuery["auditLogs"]>[number];

/** The wire's audit events as the UI's, dropping id-less entries. */
export function toAuditEvents(
  raw: AuditLogsQuery["auditLogs"] | undefined,
): AuditEvent[] {
  return (raw ?? [])
    .filter((e): e is NonNullable<RawEvent> & { id: string } => !!e?.id)
    .map((e) => ({
      id: e.id,
      timestamp: e.timestamp ?? "",
      actor: e.actor ?? "",
      actorMethod: e.actorMethod ?? "",
      actorId: e.actorId ?? "",
      actorEmail: e.actorEmail ?? "",
      sourceIp: e.sourceIp ?? "",
      userAgent: e.userAgent ?? "",
      instance: e.instance ?? "",
      action: e.action ?? "",
      status: e.status === "denied" ? "denied" : "success",
      resource: e.resource ?? "",
      targetName: e.targetName ?? "",
      displayNameFrom: e.displayNameFrom ?? "",
      displayNameTo: e.displayNameTo ?? "",
      domain: e.domain ?? "",
      envGroupId: e.envGroupId ?? "",
      envGroupName: e.envGroupName ?? "",
      envVarKeys: (e.envVarKeys ?? []).filter(
        (key): key is string => typeof key === "string" && key !== "",
      ),
      relation: e.relation ?? "",
      oauthClientId: e.oauthClientId ?? "",
      oauthAudience: e.oauthAudience ?? "",
      oauthScopes: scopes(e.oauthScopes),
      oauthGrantScopes: scopes(e.oauthGrantScopes),
    }));
}

/** A wire scope list without its null and empty entries. */
function scopes(raw: (string | null)[] | null | undefined): string[] {
  return (raw ?? []).filter(
    (scope): scope is string => typeof scope === "string" && scope !== "",
  );
}
