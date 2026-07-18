import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { UsagePage } from "@/features/usage/components/usage-page";

export const Route = createFileRoute("/usage")({
  component: UsagePage,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("usage.pageTitle", match),
});
