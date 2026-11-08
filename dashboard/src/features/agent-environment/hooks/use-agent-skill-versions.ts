import { useCallback, useState } from "react";
import { useApolloClient, useQuery } from "@apollo/client/react";
import {
  AgentSkillVersionDocument,
  AgentSkillVersionsDocument,
  type AgentSkillVersionQuery,
  type AgentSkillVersionsQuery,
} from "@/graphql/definitions";

/** One saved version as the list carries it: no content, no diff. */
export type AgentSkillVersionSummary =
  AgentSkillVersionsQuery["agentSkillVersions"]["versions"][number];
/** One version read on its own: content plus the diff against the previous. */
export type AgentSkillVersionDetail =
  AgentSkillVersionQuery["agentSkillVersion"];

const PAGE_SIZE = 20;

export interface UseAgentSkillVersionsResult {
  /** Newest first, every page loaded so far. */
  versions: AgentSkillVersionSummary[];
  /** Loading the first page — the skeleton gate. */
  loading: boolean;
  error: Error | undefined;
  retry: () => void;
  hasMore: boolean;
  loadingMore: boolean;
  loadMore: () => Promise<void>;
}

/** A skill's saved versions, newest first, paged by cursor (ADR099 D3). */
export function useAgentSkillVersions(
  ownerId: string,
  skillId: string,
): UseAgentSkillVersionsResult {
  const client = useApolloClient();
  const { data, loading, error, refetch } = useQuery(
    AgentSkillVersionsDocument,
    {
      variables: { ownerId, id: skillId, limit: PAGE_SIZE },
      fetchPolicy: "network-only",
    },
  );
  const [more, setMore] = useState<{
    versions: AgentSkillVersionSummary[];
    cursor: string | null;
  } | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);

  const first = data?.agentSkillVersions;
  const cursor = more ? more.cursor : (first?.nextCursor ?? null);

  const loadMore = useCallback(async () => {
    if (cursor == null) return;
    setLoadingMore(true);
    try {
      const res = await client.query({
        query: AgentSkillVersionsDocument,
        variables: { ownerId, id: skillId, cursor, limit: PAGE_SIZE },
        fetchPolicy: "network-only",
      });
      const page = res.data?.agentSkillVersions;
      if (page) {
        setMore((prev) => ({
          versions: [...(prev?.versions ?? []), ...page.versions],
          cursor: page.nextCursor ?? null,
        }));
      }
    } finally {
      setLoadingMore(false);
    }
  }, [client, cursor, ownerId, skillId]);

  const retry = useCallback(() => {
    setMore(null);
    void refetch();
  }, [refetch]);

  return {
    versions: [...(first?.versions ?? []), ...(more?.versions ?? [])],
    loading: loading && first == null,
    error,
    retry,
    hasMore: cursor != null,
    loadingMore,
    loadMore,
  };
}

export interface UseAgentSkillVersionResult {
  version: AgentSkillVersionDetail | null;
  loading: boolean;
  error: Error | undefined;
  retry: () => void;
}

/** One saved version with its content and its diff (ADR099 D3). */
export function useAgentSkillVersion(
  ownerId: string,
  skillId: string,
  version: number | null,
): UseAgentSkillVersionResult {
  const { data, loading, error, refetch } = useQuery(
    AgentSkillVersionDocument,
    {
      variables: { ownerId, id: skillId, version: version ?? 0 },
      skip: version == null,
      fetchPolicy: "cache-first",
    },
  );
  const detail = data?.agentSkillVersion ?? null;
  return {
    version: detail != null && detail.version === version ? detail : null,
    loading: version != null && loading && detail == null,
    error,
    retry: () => void refetch(),
  };
}
