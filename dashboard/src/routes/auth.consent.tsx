import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import type { ConsentView } from "@/common/server-fn/hydra-consent";
import ConsentPage from "@/features/auth/pages/consent-page";

// OAuth2 consent endpoint (docs/ADR012-auth.md §7, w4/m9 + w4/m16): Hydra's
// `urls.consent` points here. The GET handler runs first and answers headlessly
// whenever it can — a trusted/remembered client is auto-accepted and bounced
// straight back to Hydra, a stale challenge degrades to home — and only defers
// to this route's component (via `next`) when a third-party client needs the
// signed-in human to decide. The POST handler takes that decision. Both live
// behind a dynamic import, so the Hydra-admin logic and its server-only env
// never reach the client bundle; only `ConsentView` crosses the wire.
export const Route = createFileRoute("/auth/consent")({
  server: {
    handlers: ({ createHandlers }) =>
      createHandlers({
        GET: async ({ request, next }) => {
          const { handleConsent } =
            await import("@/common/server-fn/hydra-consent");
          const result = await handleConsent(request);
          return result instanceof Response
            ? result
            : next({ context: { consent: result } });
        },
        POST: async ({ request }) => {
          const { handleConsentDecision } =
            await import("@/common/server-fn/hydra-consent");
          return handleConsentDecision(request);
        },
      }),
  },
  validateSearch: (search: Record<string, unknown>) => ({
    consent_challenge:
      typeof search.consent_challenge === "string"
        ? search.consent_challenge
        : undefined,
  }),
  // The consent view exists only on a document request — it is the GET handler's
  // deferred context. Arriving by client-side navigation (the login-first bounce
  // lands here that way) yields none, which the page turns back into a document
  // load so the handler actually runs.
  loader: ({ serverContext }): { consent: ConsentView | null } => ({
    consent:
      (serverContext as { consent?: ConsentView } | undefined)?.consent ?? null,
  }),
  component: ConsentPage,
  head: ({ match }) => translatedTitleHead("auth.consentTitle", match),
});
