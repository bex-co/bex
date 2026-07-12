import { createFileRoute } from "@tanstack/react-router";

// Headless OAuth2 consent endpoint (docs/ADR012-auth.md, w4/m9): Hydra's
// `urls.consent` points here. Server-only — no component, no UI; the handler
// auto-accepts trusted/remembered consent requests and bounces the browser
// back to Hydra. The dynamic import keeps the Hydra-admin logic (and its
// server-only env) out of the client bundle.
export const Route = createFileRoute("/auth/consent")({
  server: {
    handlers: {
      GET: async ({ request }) => {
        const { handleConsent } =
          await import("@/common/server-fn/hydra-consent");
        return handleConsent(request);
      },
    },
  },
});
