import { createFileRoute, redirect } from "@tanstack/react-router";
import { KeyValueInfoSkeleton } from "@/common/components/route-skeletons";
import { KeyValueInfoPage } from "@/features/keyvalue/pages/key-value-info-page";
import { keyValuePageHead } from "@/features/keyvalue/lib/key-value-page-head";
import { keyValuePageRoute } from "@/features/keyvalue/components/key-value-nav";

/**
 * The bare `/keyvalue/<id>` shows Info, as Render's `/r/<id>` does (w3/m649).
 * It also takes the old single page's tabs to their page: `?tab=logs` (a
 * project log view's View in context minted it, and bookmarks keep it) and
 * `?tab=metrics`, with their range and filter keys. Every Overview anchor
 * names a card still on Info, so a hash stays here.
 */
export const Route = createFileRoute("/keyvalue/$keyValueId/")({
  head: keyValuePageHead("keyvalue.navInfo"),
  component: KeyValueInfoPage,
  pendingComponent: KeyValueInfoSkeleton,
  validateSearch: (
    search: Record<string, unknown>,
  ): { tab?: "logs" | "metrics" } => ({
    tab:
      search.tab === "logs" || search.tab === "metrics"
        ? search.tab
        : undefined,
  }),
  beforeLoad: ({ params, search, location }) => {
    if (search.tab) {
      throw redirect({
        to: keyValuePageRoute(`/${search.tab}`),
        params,
        search: ({ tab: _tab, ...rest }: Record<string, unknown>) => rest,
        hash: location.hash || undefined,
        replace: true,
      });
    }
  },
});
