export interface NewServiceSearch {
  projectId?: string;
  environmentId?: string;
}

/** Accept only nonempty string ids from contextual service-create links. */
export function parseNewServiceSearch(
  search: Record<string, unknown>,
): NewServiceSearch {
  return {
    ...(typeof search.projectId === "string" && search.projectId
      ? { projectId: search.projectId }
      : {}),
    ...(typeof search.environmentId === "string" && search.environmentId
      ? { environmentId: search.environmentId }
      : {}),
  };
}
