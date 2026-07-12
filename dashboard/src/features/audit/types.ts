// bex-native projection of bex-api's audit-log surface (backend/internal/audit
// — w4/m10; docs/ADR006-bex-api.md § Audit log). `status` is the wire's "success" |
// "denied" (internal/audit/graphql.go's renderStatus), narrowed here so the UI
// never has to string-compare the raw value.

export interface AuditEvent {
  id: string;
  timestamp: string;
  actor: string;
  actorMethod: string;
  action: string;
  status: "success" | "denied";
  resource: string;
}
