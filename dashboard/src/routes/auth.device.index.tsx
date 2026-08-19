import { createFileRoute } from "@tanstack/react-router";
import type { DeviceView } from "@/common/server-fn/hydra-device";
import DeviceConfirmPage from "@/features/auth/pages/device-confirm-page";

// The index route for the /auth/device layout (auth.device.tsx) — matches
// the exact `/auth/device` URL (no further path segment) and renders inside
// its parent's default `<Outlet/>`, leaving the sibling `/auth/device/success`
// route to render on its own.
//
// The GET/POST bridge to Hydra lives HERE, not on the layout: the route
// tree's processing hands the exact `/auth/device` path slot to whichever
// sibling's `fullPath` ends in `/` (this index route), so that is the only
// place `server.handlers` actually gets dispatched for that literal URL.
export const Route = createFileRoute("/auth/device/")({
  server: {
    handlers: ({ createHandlers }) =>
      createHandlers({
        GET: async ({ request, next }) => {
          const { handleDeviceVerification } =
            await import("@/common/server-fn/hydra-device");
          const result = await handleDeviceVerification(request);
          return result instanceof Response
            ? result
            : next({ context: { device: result } });
        },
        POST: async ({ request }) => {
          const { handleDeviceConfirm } =
            await import("@/common/server-fn/hydra-device");
          return handleDeviceConfirm(request);
        },
      }),
  },
  validateSearch: (search: Record<string, unknown>) => ({
    user_code:
      typeof search.user_code === "string" ? search.user_code : undefined,
    device_challenge:
      typeof search.device_challenge === "string"
        ? search.device_challenge
        : undefined,
  }),
  // The device view exists only on a document request — it is the GET
  // handler's deferred context. Arriving by client-side navigation (the
  // login-first bounce lands here that way, same as the consent route) yields
  // none, which the page turns back into a document load so the handler
  // actually runs.
  loader: ({ serverContext }): { device: DeviceView | null } => ({
    device:
      (serverContext as { device?: DeviceView } | undefined)?.device ?? null,
  }),
  component: DeviceConfirmPage,
});
