import type { AuditLogsQuery } from "@/graphql/definitions";
import type { AuditEvent } from "@/features/audit/types";

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
      action: e.action ?? "",
      status: e.status === "denied" ? "denied" : "success",
      resource: e.resource ?? "",
      targetName: e.targetName ?? "",
      displayNameFrom: e.displayNameFrom ?? "",
      displayNameTo: e.displayNameTo ?? "",
      domain: e.domain ?? "",
      envGroupId: e.envGroupId ?? "",
      envGroupName: e.envGroupName ?? "",
      relation: e.relation ?? "",
      oauthClientId: e.oauthClientId ?? "",
      oauthAudience: e.oauthAudience ?? "",
      oauthScopes: (e.oauthScopes ?? []).filter(
        (scope): scope is string => typeof scope === "string" && scope !== "",
      ),
    }));
}
