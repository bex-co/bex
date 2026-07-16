import { createFileRoute } from "@tanstack/react-router";

// Hydra's urls.device.verification points here. All work is server-side: the
// route bridges the user_code/device_challenge into Hydra, whose returned
// redirect continues through the existing Kratos login and consent routes.
export const Route = createFileRoute("/auth/device")({
  server: {
    handlers: ({ createHandlers }) =>
      createHandlers({
        GET: async ({ request }) => {
          const { handleDeviceVerification } =
            await import("@/common/server-fn/hydra-device");
          return handleDeviceVerification(request);
        },
      }),
  },
  head: () => ({ meta: [{ title: "Connect Render CLI — bex" }] }),
});
