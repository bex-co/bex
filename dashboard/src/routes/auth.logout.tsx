import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import LogoutPage from "@/features/auth/pages/logout-page";

export const Route = createFileRoute("/auth/logout")({
  component: LogoutPage,
  head: ({ match }) => translatedTitleHead("auth.logoutTitle", match),
});
