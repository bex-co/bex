// One read of a per-resource action projection (ADR087 / w6/m143), shared by
// the projection hooks and the dispatch recheck so both ask the same question
// the same way. Datastores joined in w4/m578.

import type { ApolloClient } from "@apollo/client";
import type { DocumentNode } from "graphql";
import {
  DatabaseActionsDocument,
  DeployActionsDocument,
  KeyValueActionsDocument,
  ServerActionsDocument,
} from "@/graphql/definitions";
import type { ResourceActionInput } from "@/features/capabilities/lib/resource-actions";

export type ProjectionField =
  "serverActions" | "deployActions" | "databaseActions" | "keyValueActions";

const DOCUMENTS: Record<ProjectionField, DocumentNode> = {
  serverActions: ServerActionsDocument,
  deployActions: DeployActionsDocument,
  databaseActions: DatabaseActionsDocument,
  keyValueActions: KeyValueActionsDocument,
};

/**
 * An uncached read of `field`'s decisions for `resourceId` in the selected
 * workspace, or undefined when the response carries no decision list. Every
 * failure rejects: callers read both as "couldn't check".
 */
export async function queryProjection(
  client: Pick<ApolloClient, "query">,
  field: ProjectionField,
  resourceId: string,
  workspaceId: string,
  signal: AbortSignal,
): Promise<ResourceActionInput[] | undefined> {
  const response = await client.query({
    query: DOCUMENTS[field],
    variables:
      field === "deployActions"
        ? { serviceId: resourceId, ownerId: workspaceId }
        : { id: resourceId, ownerId: workspaceId },
    fetchPolicy: "no-cache",
    errorPolicy: "none",
    context: { fetchOptions: { signal }, queryDeduplication: false },
  });
  const rows = (
    response.data as Record<string, ResourceActionInput[]> | undefined
  )?.[field];
  return Array.isArray(rows) ? rows : undefined;
}
