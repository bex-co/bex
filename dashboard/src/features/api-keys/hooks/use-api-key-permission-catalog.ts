import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  ApiKeyPermissionCatalogDocument,
  type ApiKeyPermissionCatalogQuery,
} from "@/graphql/definitions";
import {
  isPermissionLevel,
  toMemberRole,
} from "@/features/api-keys/lib/permissions";
import type {
  PermissionAreaView,
  PermissionLevelView,
} from "@/features/api-keys/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

type RawLevel =
  ApiKeyPermissionCatalogQuery["apiKeyPermissionCatalog"][number]["permissions"][number]["levels"][number];

/** One level, narrowed to the picker's types; null for an unknown level. */
function toLevel(level: RawLevel): PermissionLevelView | null {
  if (!isPermissionLevel(level.level)) return null;
  return {
    level: level.level,
    name: level.name,
    grantable: level.grantable,
    requiredRole: toMemberRole(level.requiredRole),
    actions: level.actions.map(({ id, name, bex }) => ({ id, name, bex })),
  };
}

// Areas, permissions and levels stay in bex-api's order: Render's areas, then
// bex's own (Cloud agents, Git), and each permission's levels narrowest first.
function toAreas(
  raw: ApiKeyPermissionCatalogQuery["apiKeyPermissionCatalog"] | undefined,
): PermissionAreaView[] | undefined {
  return raw?.map((area) => ({
    id: area.id,
    name: area.name,
    bex: area.bex,
    permissions: area.permissions.map((permission) => ({
      id: permission.id,
      name: permission.name,
      levels: permission.levels
        .map(toLevel)
        .filter((level): level is PermissionLevelView => level !== null),
    })),
  }));
}

export interface UseApiKeyPermissionCatalogResult {
  /** The picker's areas; undefined until they load (or when they fail). */
  areas: PermissionAreaView[] | undefined;
  error: Error | undefined;
  /** Re-reads the catalog in place (the picker's Try again). */
  refetch: () => Promise<unknown>;
}

/**
 * Reads bex-api's `apiKeyPermissionCatalog` (w9/m496) for the switcher's
 * workspace: Render's permission tables with, per level, whether the viewer's
 * role may grant it. Its callers mount only while a picker is open. The areas
 * and permissions are stored inline under the query, keyed by its workspace
 * (`common/apollo/cache.ts`), since `grantable` is per viewer and workspace.
 */
export function useApiKeyPermissionCatalog(): UseApiKeyPermissionCatalogResult {
  const { currentWorkspaceId } = useWorkspace();
  const { data, error, refetch } = useQuery(ApiKeyPermissionCatalogDocument, {
    variables: { ownerId: currentWorkspaceId },
    skip: currentWorkspaceId == null,
    fetchPolicy: "cache-and-network",
  });

  const areas = useMemo(() => toAreas(data?.apiKeyPermissionCatalog), [data]);

  return { areas, error, refetch };
}
