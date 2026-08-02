import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { EnvironmentsDocument } from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";

export interface EnvironmentIPAllowListEntry {
  cidrBlock: string;
  description: string;
}

export interface EnvironmentView {
  id: string;
  projectId: string;
  name: string;
  ownerId: string;
  createdAt: string | null;
  serviceIds: string[];
  databaseIds: string[];
  keyValueIds: string[];
  envGroupIds: string[];
  /** Render's protectedStatus (w6/m19): "protected" or "unprotected". */
  protectedStatus: string;
  networkIsolationEnabled: boolean;
  ipAllowListEntries: EnvironmentIPAllowListEntry[];
}

export interface UseEnvironmentsResult {
  environments: EnvironmentView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the query (fire-and-forget; callers refresh after a create/rename/delete/assign). */
  refetch: () => Promise<unknown>;
}

/**
 * Reads the environments under one project (docs/ADR032-environments.md — bex
 * extension, w1/m32). Environments are project-scoped, so this takes a
 * `projectId` rather than a workspace id and skips the query until it resolves.
 */
export function useEnvironments(
  projectId: string | null,
): UseEnvironmentsResult {
  const resolved = projectId != null && projectId !== "";
  const { data, loading, error, refetch } = useQuery(EnvironmentsDocument, {
    variables: { projectId: projectId! },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const environments = useMemo((): EnvironmentView[] => {
    if (!data?.environments) return [];
    return data.environments
      .filter((e): e is NonNullable<typeof e> => e != null)
      .map((e) => ({
        id: e.id ?? "",
        projectId: e.projectId ?? "",
        name: e.name ?? "",
        ownerId: e.ownerId ?? "",
        createdAt: e.createdAt ?? null,
        serviceIds: (e.serviceIds ?? []).filter((s): s is string => s != null),
        databaseIds: (e.databaseIds ?? []).filter(
          (s): s is string => s != null,
        ),
        keyValueIds: (e.keyValueIds ?? []).filter(
          (s): s is string => s != null,
        ),
        envGroupIds: (e.envGroupIds ?? []).filter(
          (s): s is string => s != null,
        ),
        protectedStatus: e.protectedStatus ?? "unprotected",
        networkIsolationEnabled: e.networkIsolationEnabled ?? false,
        ipAllowListEntries:
          e.ipAllowListEntries != null
            ? e.ipAllowListEntries
                .filter((entry): entry is NonNullable<typeof entry> => {
                  return entry != null && entry.cidrBlock !== "";
                })
                .map((entry) => ({
                  cidrBlock: entry.cidrBlock,
                  description: entry.description ?? "",
                }))
            : (e.ipAllowList ?? [])
                .filter((cidr): cidr is string => cidr != null && cidr !== "")
                .map((cidrBlock) => ({ cidrBlock, description: "" })),
      }));
  }, [data]);

  return { environments, loading: !resolved || loading, error, refetch };
}
