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
}

/** A freshly registered endpoint — the one and only time its secret is available. */
export interface CreatedWebhookEndpoint {
  id: string;
  name: string;
  secret: string;
}

export type WebhookDeliveryStatus = "pending" | "delivered" | "failed";

export interface WebhookDeliveryView {
  id: string;
  eventType: string;
  serviceId: string;
  status: WebhookDeliveryStatus;
  attemptCount: number;
  /** Last attempt's HTTP status; 0 = transport error or not yet attempted. */
  lastStatusCode: number;
  lastError: string;
  /** UTF-8-safe, server-bounded prefix of the latest endpoint response. */
  responseBody: string;
  /** First delivery attempt time; stable across retries. */
  sentAt: string | null;
  nextAttemptAt: string | null;
  deliveredAt: string | null;
  createdAt: string | null;
  /** Opaque keyset cursor — echo the last row's back to page further. */
  cursor: string;
}
