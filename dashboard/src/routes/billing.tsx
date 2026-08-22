import { createFileRoute } from "@tanstack/react-router";
import { ListPageSkeleton } from "@/common/components/detail-skeletons";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  translatedTitleHead,
  titleLoaderFetchPolicy,
} from "@/common/lib/document-head";
import { prefetchInParallel } from "@/common/lib/prefetch";
import { UsageDocument } from "@/graphql/definitions";
import { UsagePage } from "@/features/usage/components/usage-page";

export const Route = createFileRoute("/billing")({
  staticData: { chrome: true },
  component: UsagePage,
  pendingComponent: ListPageSkeleton,
  beforeLoad: requireAuth(),
  // Match `useUsage()` with no period arg — current-month cache key.
  loader: ({ context, cause }) => {
    const ownerId = context.workspaceId;
    if (ownerId == null) return;
    return prefetchInParallel([
      () =>
        context.client.query({
          query: UsageDocument,
          variables: { ownerId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
    ]);
  },
  head: ({ match }) => translatedTitleHead("usage.pageTitle", match),
});
