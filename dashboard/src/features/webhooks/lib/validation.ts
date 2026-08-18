import type { WebhookFormField } from "@/features/webhooks/lib/errors";

export type WebhookFormErrors = Partial<Record<WebhookFormField, string>>;

export interface WebhookValidationInput {
  name: string;
  url: string;
  selectedEventCount: number;
  existingNames: string[];
  /** The endpoint's current name is allowed on Settings. */
  currentName?: string;
}

export function isValidWebhookHTTPSURL(value: string): boolean {
  try {
    const parsed = new URL(value.trim());
    return (
      parsed.protocol === "https:" &&
      parsed.host !== "" &&
      parsed.username === "" &&
      parsed.password === ""
    );
  } catch {
    return false;
  }
}

/** Client-side preflight. The server remains authoritative under races. */
export function validateWebhookForm(
  input: WebhookValidationInput,
  messages: {
    nameRequired: string;
    nameDuplicate: string;
    urlRequired: string;
    urlHTTPS: string;
    eventsRequired: string;
  },
): WebhookFormErrors {
  const errors: WebhookFormErrors = {};
  const name = input.name.trim();
  if (!name) {
    errors.name = messages.nameRequired;
  } else {
    const normalized = name.toLocaleLowerCase();
    const current = input.currentName?.trim().toLocaleLowerCase();
    if (
      normalized !== current &&
      input.existingNames.some(
        (candidate) => candidate.trim().toLocaleLowerCase() === normalized,
      )
    ) {
      errors.name = messages.nameDuplicate;
    }
  }
  if (!input.url.trim()) errors.url = messages.urlRequired;
  else if (!isValidWebhookHTTPSURL(input.url)) errors.url = messages.urlHTTPS;
  if (input.selectedEventCount === 0) {
    errors.events = messages.eventsRequired;
  }
  return errors;
}

export const webhookFieldOrder: WebhookFormField[] = [
  "name",
  "url",
  "events",
  "form",
];

export function clearWebhookFormError(
  errors: WebhookFormErrors,
  field: WebhookFormField,
): WebhookFormErrors {
  if (!errors[field]) return errors;
  const next = { ...errors };
  delete next[field];
  return next;
}
