import { createFileRoute } from "@tanstack/react-router";

// Readiness endpoint for the dashboard Deployment (w1/m52): 200 once the pod
// has reached bex-api at least once, 503 before — so a freshly rolled pod
// doesn't take traffic while its SSR upstream is still unreachable (inbox
// 036's post-roll error page). Latched on first success; the liveness probe
// stays on `/`. Dynamic-import discipline as `/api/sessions`: handlers always
// return a Response and keep server code out of the client bundle.
export const Route = createFileRoute("/healthz")({
  server: {
    handlers: ({ createHandlers }) =>
      createHandlers({
        GET: async () => {
          const { readinessCheck } = await import("@/common/server-fn/health");
          return readinessCheck();
        },
      }),
  },
});
