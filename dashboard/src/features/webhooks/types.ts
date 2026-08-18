// bex-native projections of bex-api's outbound-webhook surface
// (backend/internal/webhooks, w3/m11). An endpoint's signing secret is never a
// field on WebhookEndpointView — the list/get queries don't even request it;
// the only place a secret ever exists client-side is CreatedWebhookEndpoint,
// returned by the create mutation exactly once and held in dialog-local state.

export interface WebhookEndpointView {
  id: string;
  name: string;
  url: string;
  eventTypes: string[];
  enabled: boolean;
  /** Why the endpoint is disabled — "" while enabled; the delivery worker's
   * auto-disable writes its own reason here. */
  disabledReason: string;
  createdAt: string | null;
  /** Stored creator subject — resolved through the workspace member API for
   * display; "" when none was recorded. List rows leave it empty. */
  createdBy: string;
  /** Newest completed immutable attempt, populated on list rows. */
  latestStatus?: WebhookDeliveryStatus | "";
  latestSentAt?: string | null;
  /** Logical notification state for the newest attempt. */
  latestParentStatus?: WebhookDeliveryStatus | "";
}

/** A freshly registered endpoint — the one and only time its secret is available. */
export interface CreatedWebhookEndpoint {
  id: string;
  name: string;
  secret: string;
}

export type WebhookDeliveryStatus = "pending" | "delivered" | "failed";

export interface WebhookDeliveryView {
  /** Immutable identity of this one network exchange/reservation. */
  id: string;
  /** Stable source event identity shared by every attempt for a notification. */
  eventId: string;
  eventType: string;
  serviceId: string;
  status: WebhookDeliveryStatus;
  /** One-based send number within the logical endpoint notification. */
  attemptNumber: number;
  /** This attempt's HTTP status; 0 = transport error or not yet attempted. */
  statusCode: number;
  transportError: string;
  /** UTF-8-safe, server-bounded evidence for this immutable attempt. */
  responseBody: string;
  requestBody: string;
  sentAt: string | null;
  /** Parent notification scheduling state, separate from attempt outcome. */
  nextAttemptAt: string | null;
  parentStatus: WebhookDeliveryStatus;
  /** Opaque keyset cursor — echo the last row's back to page further. */
  cursor: string;
}
