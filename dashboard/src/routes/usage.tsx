import { createFileRoute } from "@tanstack/react-router";
import { ListPageSkeleton } from "@/common/components/detail-skeletons";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { UsagePage } from "@/features/usage/components/usage-page";

export const Route = createFileRoute("/usage")({
  staticData: { chrome: true },
  component: UsagePage,
  pendingComponent: ListPageSkeleton,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("usage.pageTitle", match),
});
