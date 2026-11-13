import { DeploysListPage } from "@/features/deploys/components/deploys-list-page";
import {
  deployListFiltersFromSearch,
  deployListFiltersToSearch,
  type DeployListSearch,
} from "@/features/deploys/lib/deploy-list-search";

/** A service's deploy history bound to its route's URL search: the Deploys
 *  tab, a static site's history, and a cron job's Builds page (w2/m360). */
export function DeployHistoryView({
  serviceId,
  search,
  replaceSearch,
  titleKey,
}: {
  serviceId: string;
  search: DeployListSearch;
  /** Writes the next search with `replace`, so typing adds no history entries. */
  replaceSearch: (next: DeployListSearch) => void;
  titleKey?: "deploys.listTitle" | "deploys.buildsTitle";
}) {
  return (
    <DeploysListPage
      // Filters belong to one service's history.
      key={serviceId}
      serviceId={serviceId}
      titleKey={titleKey}
      urlFilters={deployListFiltersFromSearch(search)}
      onFiltersChange={(filters) =>
        replaceSearch(deployListFiltersToSearch(filters))
      }
    />
  );
}
