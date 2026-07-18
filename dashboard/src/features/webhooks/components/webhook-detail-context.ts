import { createContext, useContext } from "react";
import type { UseWebhookResult } from "@/features/webhooks/hooks/use-webhook";

/**
 * The /webhook/$webhookId shell fetches the endpoint once and shares it with
 * its child routes (Activity, Settings) through this context, so the tabs
 * never re-fire the query on switch (w1/m49/t004).
 */
export const WebhookDetailContext = createContext<UseWebhookResult | null>(
  null,
);

export function useWebhookDetail(): UseWebhookResult {
  const ctx = useContext(WebhookDetailContext);
  if (!ctx) {
    throw new Error(
      "useWebhookDetail must be used inside the /webhook/$webhookId shell",
    );
  }
  return ctx;
}
