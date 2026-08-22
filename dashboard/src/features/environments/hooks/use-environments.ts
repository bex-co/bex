import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  EnvironmentsDocument,
  type EnvironmentsQuery,
} from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";

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

export interface UseEnvironmentsOptions {
  /**
   * Poll for out-of-band changes (default `true`). Pass `false` on a secondary
   * consumer mounted alongside a polling one: every `useQuery` gets its own
   * timer, and two timers reschedule off their own responses, so they drift
   * apart into separate round trips instead of deduplicating.
   */
  poll?: boolean;
}

export interface UseEnvironmentsResult {
  environments: EnvironmentView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the query (fire-and-forget; callers refresh after a create/rename/delete/assign). */
  refetch: () => Promise<unknown>;
}

export function mapEnvironments(
  raw: EnvironmentsQuery["environments"] | undefined,
): EnvironmentView[] {
  return (raw ?? [])
    .filter(
      (environment): environment is NonNullable<typeof environment> =>
        environment != null,
    )
    .map((environment) => ({
      id: environment.id ?? "",
      projectId: environment.projectId ?? "",
      name: environment.name ?? "",
      ownerId: environment.ownerId ?? "",
      createdAt: environment.createdAt ?? null,
      serviceIds: (environment.serviceIds ?? []).filter(
        (id): id is string => id != null,
      ),
      databaseIds: (environment.databaseIds ?? []).filter(
        (id): id is string => id != null,
      ),
      keyValueIds: (environment.keyValueIds ?? []).filter(
        (id): id is string => id != null,
      ),
      envGroupIds: (environment.envGroupIds ?? []).filter(
        (id): id is string => id != null,
      ),
      protectedStatus: environment.protectedStatus ?? "unprotected",
      networkIsolationEnabled: environment.networkIsolationEnabled ?? false,
      ipAllowListEntries:
        environment.ipAllowListEntries != null
          ? environment.ipAllowListEntries
              .filter((entry): entry is NonNullable<typeof entry> => {
                return entry != null && entry.cidrBlock !== "";
              })
              .map((entry) => ({
                cidrBlock: entry.cidrBlock,
                description: entry.description ?? "",
              }))
          : (environment.ipAllowList ?? [])
              .filter((cidr): cidr is string => cidr != null && cidr !== "")
              .map((cidrBlock) => ({ cidrBlock, description: "" })),
    }));
}

/**
 * Reads the environments under one project (docs/ADR032-environments.md — bex
 * extension, w1/m32). Environments are project-scoped, so this takes a
 * `projectId` rather than a workspace id and skips the query until it resolves.
 */
export function useEnvironments(
  projectId: string | null,
  { poll = true }: UseEnvironmentsOptions = {},
): UseEnvironmentsResult {
  const resolved = projectId != null && projectId !== "";
  const { data, loading, error, refetch } = useQuery(EnvironmentsDocument, {
    variables: { projectId: projectId! },
    skip: !resolved,
    // Chrome consumers pass `poll: false` and rely on a warm cache (service
    // detail loader / prior list visit). Polling list pages keep background
    // freshness via pollInterval either way.
    fetchPolicy: poll ? "cache-and-network" : PRIMED_FETCH_POLICY,
    errorPolicy: "all",
    pollInterval: poll ? RESOURCE_POLL_INTERVAL_MS : 0,
    skipPollAttempt: skipPollWhenHidden,
  });

  const environments = useMemo(
    () => mapEnvironments(data?.environments),
    [data],
  );

  return { environments, loading: !resolved || loading, error, refetch };
}
