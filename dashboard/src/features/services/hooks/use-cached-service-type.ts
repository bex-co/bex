import { useApolloClient } from "@apollo/client/react";
import { ServerDocument, ServicesDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * A service's type as already held in Apollo's cache — its own `server(id)`
 * read, else the workspace `services` list (sidebar/breadcrumb warm) — without
 * issuing a request. Loading skeletons use it to shape type-dependent regions
 * (the plan picker's Free group); null means the type is still undecided.
 */
export function useCachedServiceType(serviceId: string): string | null {
  const client = useApolloClient();
  const { currentWorkspaceId } = useWorkspace();
  // Partial reads: only `type` matters, so a document missing an unrelated
  // field still answers.
  const server = client.cache.readQuery({
    query: ServerDocument,
    variables: { id: serviceId },
    returnPartialData: true,
  })?.server;
  if (server?.type) return server.type;
  if (currentWorkspaceId == null) return null;
  const listed = client.cache
    .readQuery({
      query: ServicesDocument,
      variables: { ownerId: currentWorkspaceId },
      returnPartialData: true,
    })
    ?.services?.find((service) => service?.id === serviceId);
  return listed?.type ?? null;
}
