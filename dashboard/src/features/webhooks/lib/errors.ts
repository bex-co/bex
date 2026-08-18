import { CombinedGraphQLErrors } from "@apollo/client/errors";

export type WebhookFormField = "name" | "url" | "events" | "form";

/** A stable backend refusal anchored to the field the user can fix. */
export class WebhookMutationError extends Error {
  readonly code: string;
  readonly field: WebhookFormField;

  constructor(code: string, message: string, field: WebhookFormField) {
    super(message);
    this.name = "WebhookMutationError";
    this.code = code;
    this.field = field;
  }
}

const metadataByCode: Record<
  string,
  { field: WebhookFormField; messageKey: string }
> = {
  WEBHOOK_NAME_INVALID: {
    field: "name",
    messageKey: "webhooks.validation.nameRequired",
  },
  WEBHOOK_NAME_CONFLICT: {
    field: "name",
    messageKey: "webhooks.validation.nameDuplicate",
  },
  WEBHOOK_URL_INVALID: {
    field: "url",
    messageKey: "webhooks.validation.urlHTTPS",
  },
  WEBHOOK_EVENT_FILTER_INVALID: {
    field: "events",
    messageKey: "webhooks.validation.eventsInvalid",
  },
  WEBHOOK_EVENT_FILTER_REQUIRED: {
    field: "events",
    messageKey: "webhooks.validation.eventsInvalid",
  },
  WEBHOOK_ENDPOINT_LIMIT: {
    field: "form",
    messageKey: "webhooks.errors.endpointLimit",
  },
  WEBHOOK_ENDPOINT_DISABLED: {
    field: "form",
    messageKey: "webhooks.errors.endpointDisabled",
  },
  WEBHOOK_DELIVERY_PENDING: {
    field: "form",
    messageKey: "webhooks.errors.deliveryPending",
  },
  WEBHOOK_ENDPOINT_NOT_FOUND: {
    field: "form",
    messageKey: "webhooks.errors.staleState",
  },
  WEBHOOK_DELIVERY_NOT_FOUND: {
    field: "form",
    messageKey: "webhooks.errors.staleState",
  },
};

function normalizedField(value: unknown): WebhookFormField | undefined {
  if (value === "name" || value === "url") return value;
  if (value === "eventTypes" || value === "eventFilter") return "events";
  return undefined;
}

/** Preserve Apollo transport errors, but type every named webhook refusal. */
export function toWebhookMutationError(error: unknown): unknown {
  if (!CombinedGraphQLErrors.is(error)) return error;
  for (const item of error.errors) {
    const code = item.extensions?.["code"];
    if (typeof code !== "string" || !code.startsWith("WEBHOOK_")) continue;
    return new WebhookMutationError(
      code,
      item.message,
      normalizedField(item.extensions?.["field"]) ??
        metadataByCode[code]?.field ??
        "form",
    );
  }
  return error;
}

export function webhookErrorMessageKey(error: unknown): string | null {
  const normalized = toWebhookMutationError(error);
  return normalized instanceof WebhookMutationError
    ? (metadataByCode[normalized.code]?.messageKey ?? null)
    : null;
}
