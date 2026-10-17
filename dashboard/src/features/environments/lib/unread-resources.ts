import type { ProjectResourceFamily } from "@/features/projects/lib/resource-filter";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";

/** Which resource lists have not answered, for one card (w4/m227). */
export interface UnreadResources {
  pending: ProjectResourceFamily[];
  failed: ProjectResourceFamily[];
  onRetry: () => void;
  retrying: boolean;
}

export const NOTHING_UNREAD: UnreadResources = {
  pending: [],
  failed: [],
  onRetry: () => undefined,
  retrying: false,
};

/** Families whose rows are unknown: still loading, or failed. */
export function unreadFamilies(
  unread: UnreadResources,
): ProjectResourceFamily[] {
  return [...unread.pending, ...unread.failed];
}

/** An environment's member ids by resource family. */
export function environmentMemberIds(
  environment: EnvironmentView,
): Record<ProjectResourceFamily, string[]> {
  return {
    services: environment.serviceIds,
    databases: environment.databaseIds,
    keyvalues: environment.keyValueIds,
    envgroups: environment.envGroupIds,
  };
}
