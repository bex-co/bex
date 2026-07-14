import { createFileRoute } from "@tanstack/react-router";

// Settings → Security & Compliance "Active sessions" card (w4/m18) fetches
// this endpoint directly (not via router navigation) to list the signed-in
// user's Kratos sessions and sign one (or every other one) out. Handlers
// always return a Response, mirroring `/auth/consent`'s dynamic-import
// discipline so Kratos client code stays out of the client bundle.
export const Route = createFileRoute("/api/sessions")({
  server: {
    handlers: ({ createHandlers }) =>
      createHandlers({
        GET: async ({ request }) => {
          const { listSessions } = await import(
            "@/common/server-fn/kratos-sessions"
          );
          return listSessions(request);
        },
        POST: async ({ request }) => {
          const { revokeSession } = await import(
            "@/common/server-fn/kratos-sessions"
          );
          return revokeSession(request);
        },
      }),
  },
});
