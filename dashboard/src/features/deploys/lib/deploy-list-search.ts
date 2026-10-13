// The status filter's vocabulary: the deploy-status enum the store writes
// (store.Deploy*), labeled by the same keys the badges use.
export const DEPLOY_STATUS_FILTERS = [
  "created",
  "queued",
  "build_in_progress",
  "build_failed",
  "pre_deploy_in_progress",
  "pre_deploy_failed",
  "update_in_progress",
  "update_failed",
  "live",
  "deactivated",
  "canceled",
] as const;

/** The deploy history's filters; an empty value means "not filtered". */
export interface DeployListFilters {
  status: string;
  search: string;
}

export const NO_DEPLOY_LIST_FILTERS: DeployListFilters = {
  status: "",
  search: "",
};

/** The history's URL search: its filters, so Back from a deploy restores them. */
export interface DeployListSearch {
  status?: string;
  q?: string;
}

export function parseDeployListSearch(
  search: Record<string, unknown>,
): DeployListSearch {
  // Both keys come back, undefined when absent or rejected: the router merges
  // this over the raw search, so an omitted `?status=bogus` would survive.
  const status = (DEPLOY_STATUS_FILTERS as readonly unknown[]).includes(
    search.status,
  )
    ? (search.status as string)
    : undefined;
  // Numeric-looking text arrives parsed (`?q=123`); it is still the search.
  const q =
    typeof search.q === "string" || typeof search.q === "number"
      ? String(search.q) || undefined
      : undefined;
  return { status, q };
}

export function deployListFiltersFromSearch(
  search: DeployListSearch,
): DeployListFilters {
  return { status: search.status ?? "", search: search.q ?? "" };
}

/** Every key written explicitly, so a cleared filter actually leaves the URL. */
export function deployListFiltersToSearch(
  filters: DeployListFilters,
): DeployListSearch {
  return {
    status: filters.status || undefined,
    q: filters.search || undefined,
  };
}
