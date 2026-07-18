import { createFileRoute } from "@tanstack/react-router";
import DeviceSuccessPage from "@/features/auth/pages/device-success-page";
import { translatedTitleHead } from "@/common/lib/document-head";

// Browser-only landing page after Hydra has bound login + consent to the
// device request. The CLI receives tokens independently through its poll loop.
export const Route = createFileRoute("/auth/device/success")({
  component: DeviceSuccessPage,
  head: ({ match }) => translatedTitleHead("auth.deviceSuccessTitle", match),
});
