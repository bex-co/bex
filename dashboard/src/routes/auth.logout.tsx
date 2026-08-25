import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import LogoutPage from "@/features/auth/pages/logout-page";
import { LogoutRouteSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/auth/logout")({
  component: LogoutPage,
  pendingComponent: LogoutRouteSkeleton,
  head: ({ match }) => translatedTitleHead("auth.logoutTitle", match),
});
