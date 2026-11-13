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
  /** Target display name (backend migration 0038); "" on pre-0038 rows — the
   *  row falls back to the raw resource id. */
  targetName: string;
  /** A rename's label before and after; "" on every other row (w5/183). */
  displayNameFrom: string;
  displayNameTo: string;
  /** The hostname a custom-domain verb changed; "" on every other row. */
  domain: string;
  /** The env group a link or unlink changed; "" on every other row. */
  envGroupId: string;
  envGroupName: string;
  /** RelCan… the decision was made against; "" on pre-0088 and system rows. */
  relation: string;
  /** Verified OAuth client id; "" on session/machine/pre-0088 rows. */
  oauthClientId: string;
  /** Accepted API audience; "" when absent. */
  oauthAudience: string;
  /** Canonical capability scopes; empty when absent. */
  oauthScopes: string[];
}
