import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import type {
  ConsentErrorCode,
  ConsentView,
} from "@/common/server-fn/hydra-consent";
import ConsentPage from "@/features/auth/pages/consent-page";
import { ConsentRouteSkeleton } from "@/common/components/route-skeletons";

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
          const { withReportedRouteError } =
            await import("@/common/lib/report-route-error");
          return withReportedRouteError(async () => {
            const { handleConsent } =
              await import("@/common/server-fn/hydra-consent");
            const result = await handleConsent(request);
            if (result instanceof Response) return result;
            // One `next()` call with a uniformly-typed context object — a
            // per-branch `next()` call forces TanStack's context inference to
            // the first branch's literal shape and rejects the second.
            const context: {
              consent: ConsentView | null;
              consentErrorCode: ConsentErrorCode | null;
              consentRequiredScopes: string[] | null;
            } =
              "errorCode" in result
                ? {
                    consent: null,
                    consentErrorCode: result.errorCode,
                    consentRequiredScopes: result.requiredScopes ?? null,
                  }
                : {
                    consent: result,
                    consentErrorCode: null,
                    consentRequiredScopes: null,
                  };
            return next({ context });
          });
        },
        POST: async ({ request }) => {
          const { withReportedRouteError } =
            await import("@/common/lib/report-route-error");
          return withReportedRouteError(async () => {
            const { handleConsentDecision } =
              await import("@/common/server-fn/hydra-consent");
            return handleConsentDecision(request);
          });
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
  loader: ({
    serverContext,
  }): {
    consent: ConsentView | null;
    consentErrorCode: ConsentErrorCode | null;
    consentRequiredScopes: string[] | null;
  } => {
    const ctx = serverContext as
      | {
          consent?: ConsentView;
          consentErrorCode?: ConsentErrorCode;
          consentRequiredScopes?: string[];
        }
      | undefined;
    return {
      consent: ctx?.consent ?? null,
      consentErrorCode: ctx?.consentErrorCode ?? null,
      consentRequiredScopes: ctx?.consentRequiredScopes ?? null,
    };
  },
  component: ConsentPage,
  pendingComponent: ConsentRouteSkeleton,
  head: ({ match }) => translatedTitleHead("auth.consentTitle", match),
});
