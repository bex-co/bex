import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";

// Hydra's urls.device.verification points here. All work is server-side: the
// route bridges the user_code/device_challenge into Hydra, whose returned
// redirect continues through the existing Kratos login and consent routes.
// The grant never pairs on a GET: a signed-out visitor is bounced through
// login first (code + challenge preserved in `next`), and every caller —
// signed in or fresh from that bounce — must confirm via the same-origin,
// session-bound POST before pairing (handleDeviceConfirm, codex-security #9).
export const Route = createFileRoute("/auth/device")({
  server: {
    handlers: ({ createHandlers }) =>
      createHandlers({
        GET: async ({ request }) => {
          const { handleDeviceVerification } = await import(
            "@/common/server-fn/hydra-device"
          );
          return handleDeviceVerification(request);
        },
        POST: async ({ request }) => {
          const { handleDeviceConfirm } = await import(
            "@/common/server-fn/hydra-device"
          );
          return handleDeviceConfirm(request);
        },
      }),
  },
  head: ({ match }) => translatedTitleHead("auth.deviceTitle", match),
});
