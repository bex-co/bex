import { createFileRoute } from "@tanstack/react-router";

// Settings → Security & Compliance "Connected agents" card (w4/m18) fetches
// this endpoint directly (not via router navigation) to list + revoke the
// signed-in user's Hydra consent sessions. Handlers always return a Response,
// mirroring `/auth/consent`'s dynamic-import discipline so the Hydra admin
// client and its env never reach the client bundle.
export const Route = createFileRoute("/api/connected-agents")({
  server: {
    handlers: ({ createHandlers }) =>
      createHandlers({
        GET: async ({ request }) => {
          const { listConnectedAgents } = await import(
            "@/common/server-fn/hydra-connected-agents"
          );
          return listConnectedAgents(request);
        },
        POST: async ({ request }) => {
          const { revokeConnectedAgent } = await import(
            "@/common/server-fn/hydra-connected-agents"
          );
          return revokeConnectedAgent(request);
        },
      }),
  },
});
