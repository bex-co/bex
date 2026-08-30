import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { AccountDeletedRouteSkeleton } from "@/common/components/route-skeletons";
import AccountDeletedPage from "@/features/auth/pages/account-deleted-page";

export const Route = createFileRoute("/auth/account-deleted")({
  component: AccountDeletedPage,
  pendingComponent: AccountDeletedRouteSkeleton,
  head: ({ match }) => translatedTitleHead("auth.accountDeletedTitle", match),
});
