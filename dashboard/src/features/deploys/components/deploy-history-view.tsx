import { AutoDeployOffBanner } from "@/features/deploys/components/auto-deploy-off-banner";
import { DeploysListPage } from "@/features/deploys/components/deploys-list-page";
import {
  autoDeployOff,
  deploysManualOnly,
} from "@/features/deploys/lib/auto-deploy-off";
import { useServer } from "@/features/services/hooks/use-server";
import type { ServiceBase } from "@/features/services/lib/service-base";
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
  base = "/services",
}: {
  serviceId: string;
  search: DeployListSearch;
  /** Writes the next search with `replace`, so typing adds no history entries. */
  replaceSearch: (next: DeployListSearch) => void;
  /** The tree the service's own Settings link stays in. */
  base?: ServiceBase;
}) {
  // The layout already holds the service, so the banner is known on the
  // first paint and never shifts the list.
  const { service } = useServer(serviceId, { poll: false });
  const list = (
    <DeploysListPage
      // Filters belong to one service's history.
      key={serviceId}
      serviceId={serviceId}
      urlFilters={deployListFiltersFromSearch(search)}
      onFiltersChange={(filters) =>
        replaceSearch(deployListFiltersToSearch(filters))
      }
    />
  );
  if (!autoDeployOff(service)) return list;
  return (
    <div className="space-y-4">
      <AutoDeployOffBanner
        serviceId={serviceId}
        branch={service?.branch ?? null}
        base={base}
        manual={deploysManualOnly(service)}
      />
      {list}
    </div>
  );
}
