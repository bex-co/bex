import { useApolloClient } from "@apollo/client/react";
import { ServerDocument, ServicesDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { isCronType } from "@/features/services/lib/service-type";
import {
  isSuspended,
  suspendedNoticeTitleKeyFor,
  type SuspendedNoticeTitleKey,
} from "@/features/services/lib/status";

/**
 * The service as already held in Apollo's cache — its own `server(id)` read,
 * else the workspace `services` list (sidebar/breadcrumb warm) — without
 * issuing a request. Both name the same normalized `Service` entity, so a
 * field either query wrote reads back here. Partial reads: a skeleton needs
 * only a field or two, so a document missing an unrelated field still
 * answers, and a field no query has written yet reads `undefined`.
 */
function useCachedService(serviceId: string) {
  const client = useApolloClient();
  const { currentWorkspaceId } = useWorkspace();
  const server = client.cache.readQuery({
    query: ServerDocument,
    variables: { id: serviceId },
    returnPartialData: true,
  })?.server;
  if (server) return server;
  if (currentWorkspaceId == null) return null;
  return (
    client.cache
      .readQuery({
        query: ServicesDocument,
        variables: { ownerId: currentWorkspaceId },
        returnPartialData: true,
      })
      ?.services?.find((service) => service?.id === serviceId) ?? null
  );
}

/**
 * A service's type from the cache. Loading skeletons use it to shape
 * type-dependent regions (the plan picker's Free group); null means the type
 * is still undecided.
 */
export function useCachedServiceType(serviceId: string): string | null {
  return useCachedService(serviceId)?.type ?? null;
}

/**
 * The service's name from the cache, which the header's loading frame lays
 * out invisibly so its first row wraps where the ready one does (w3/m555).
 * Null while no cached read has named it.
 */
export function useCachedServiceName(serviceId: string): string | null {
  return useCachedService(serviceId)?.name ?? null;
}

/** The source facts the service header draws beside its Service ID. */
export interface CachedServiceSource {
  /** It builds from a repository: the header shows the repository and branch. */
  repository: boolean;
  /** It pulls its image with a registry credential, which the header names. */
  registryCredential: boolean;
  /** A cron job: its header's action is Trigger Run and its facts are the
   *  two "Runs …" lines (w3/m555). */
  cron: boolean;
  /** While it is suspended, the title of the notice its header then has in
   *  place of any action (w3/107); null while it runs. */
  suspendedNotice: SuspendedNoticeTitleKey | null;
}

/**
 * Which source facts the header will show, from the cache, so its loading
 * frame wraps where the ready header does (w3/094): an image-backed service
 * has no repository item, and its row is a line shorter at some widths. Null
 * while no cached read has said whether the service has a repository.
 */
export function useCachedServiceSource(
  serviceId: string,
): CachedServiceSource | null {
  const service = useCachedService(serviceId);
  if (service?.repo === undefined) return null;
  return {
    repository: Boolean(service.repo),
    registryCredential: Boolean(service.registryCredentialId),
    cron: service.type != null && isCronType(service.type),
    suspendedNotice: isSuspended(service.suspended ?? null)
      ? suspendedNoticeTitleKeyFor(service.suspenders)
      : null,
  };
}
