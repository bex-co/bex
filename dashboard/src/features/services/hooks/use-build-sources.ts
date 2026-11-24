import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@apollo/client/react";
import {
  BuildSourceBuildsDocument,
  BuildSourceDocument,
  BuildSourcesDocument,
  type BuildSourceFieldsFragment,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { formatRepoLabel } from "@/features/services/lib/repo";
import {
  isNativeRuntime,
  RUNTIME_COMMANDS,
  type NativeRuntime,
} from "@/features/services/lib/runtime";

/** The picker's page size: Render's page cap. */
const BUILD_SOURCES_PAGE = 100;

/** A build source as the Source card, Update Source dialog, and create wizard show it. */
export interface BuildSourceOption {
  id: string;
  name: string;
  /** The repo URL (git) or image (image) it builds from. */
  source: string;
  kind: "git" | "image";
  /** Tracked git branch for git-backed build sources; null for image sources. */
  branch?: string | null;
  /** Declared Render region for git-backed build sources; null for image sources. */
  region?: string | null;
  /**
   * The native runtime it builds with, so a linked service supplies its own
   * start command (Render); null for a Docker or image source, which takes
   * none. Same allowlist as the backend (buildSourceLink).
   */
  nativeRuntime: NativeRuntime | null;
}

/** One build of a Build Source (Render's Build object). */
export interface BuildSourceBuildView {
  id: string;
  buildSourceId: string;
  /** Render buildRun.status: created | in_progress | succeeded | failed | canceled. */
  status: string;
  /** True when this build produced a deployable artifact (selectable in Manual Deploy). */
  succeeded: boolean;
  commitId: string | null;
  commitUrl: string | null;
  imageRef: string | null;
  imageSha: string | null;
  createdAt: string | null;
}

function toOption(b: BuildSourceFieldsFragment | null | undefined) {
  if (!b?.id) return null;
  const runtime = b.git?.runtime;
  return {
    id: b.id,
    name: b.name ?? b.id,
    source: b.git?.repoUrl ?? b.image?.imageUrl ?? "",
    kind: b.git ? "git" : "image",
    branch: b.git?.branch ?? null,
    region: b.git?.region ?? null,
    nativeRuntime: b.git && isNativeRuntime(runtime) ? runtime : null,
  } satisfies BuildSourceOption;
}

/** What a build source builds from: repo (+ optional branch) or image URL. */
export function buildSourceDisplayLabel(source: BuildSourceOption): string {
  if (source.kind !== "git") return source.source;
  const repo = formatRepoLabel(source.source);
  return source.branch ? `${repo} · ${source.branch}` : repo;
}

/** The default start command a native build source's linked service gets. */
export function defaultStartCommand(source: BuildSourceOption): string {
  return source.nativeRuntime
    ? RUNTIME_COMMANDS[source.nativeRuntime].start
    : "";
}

/**
 * Reads the workspace's build sources for the Update Source picker (w2/m254).
 * `enabled` false skips the read until the dialog opens.
 */
export function useBuildSources(enabled: boolean) {
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error, fetchMore } = useQuery(BuildSourcesDocument, {
    variables: { ownerId: currentWorkspaceId },
    skip: !enabled || currentWorkspaceId == null,
    fetchPolicy: "cache-and-network",
    nextFetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  // A full page means there may be more: fetch the next one and append it,
  // until a short page ends the list. Each cursor is fetched once, and a page
  // is merged by id, so a repeated cursor or a doubled effect cannot loop or
  // duplicate.
  const pages = data?.buildSources ?? [];
  const lastCursor = pages.at(-1)?.cursor ?? null;
  const more = pages.length > 0 && pages.length % BUILD_SOURCES_PAGE === 0;
  const fetched = useRef(new Set<string>());
  // The last cursor whose page has settled (arrived or failed).
  const [settled, setSettled] = useState<string | null>(null);
  useEffect(() => {
    if (!more || !lastCursor || loading || fetched.current.has(lastCursor))
      return;
    fetched.current.add(lastCursor);
    fetchMore({
      variables: { cursor: lastCursor },
      updateQuery: (prev, { fetchMoreResult }) => {
        const seen = new Set((prev.buildSources ?? []).map((b) => b?.id));
        return {
          ...prev,
          buildSources: [
            ...(prev.buildSources ?? []),
            ...(fetchMoreResult.buildSources ?? []).filter(
              (b) => !seen.has(b?.id),
            ),
          ],
        };
      },
    })
      .catch(() => {
        // A failed page ends the walk; the picker keeps what it has.
      })
      .finally(() => setSettled(lastCursor));
  }, [more, lastCursor, loading, fetchMore]);
  const sources = useMemo(
    () =>
      (data?.buildSources ?? []).flatMap((b) => {
        const option = toOption(b);
        return option ? [option] : [];
      }),
    [data],
  );
  // Still loading while a full last page promises a page not yet settled.
  const pending = more && lastCursor !== null && settled !== lastCursor;
  return { sources, loading: loading || pending, error };
}

/** Reads the one build source a linked service builds from (null id skips). */
export function useBuildSource(id: string | null) {
  const { data, loading } = useQuery(BuildSourceDocument, {
    variables: { id: id ?? "" },
    skip: !id,
    errorPolicy: "all",
  });
  const source = useMemo(() => toOption(data?.buildSource), [data]);
  return { source, loading };
}

/**
 * Lists recent builds for a Build Source (w2/m612 t004), used by the
 * linked-service Manual Deploy "Deploy a specific build…" picker.
 */
export function useBuildSourceBuilds(
  buildSourceId: string | null,
  enabled: boolean,
) {
  const { data, loading, error, refetch } = useQuery(
    BuildSourceBuildsDocument,
    {
      variables: { buildSourceId: buildSourceId ?? "", limit: 50 },
      skip: !enabled || !buildSourceId,
      fetchPolicy: "cache-and-network",
      nextFetchPolicy: "cache-first",
      errorPolicy: "all",
    },
  );
  const builds = useMemo<BuildSourceBuildView[]>(
    () =>
      (data?.builds ?? []).flatMap((b) => {
        if (!b?.id) return [];
        const status = b.buildRun?.status ?? (b.image ? "succeeded" : "");
        return [
          {
            id: b.id,
            buildSourceId: b.buildSourceId ?? buildSourceId ?? "",
            status,
            succeeded: status === "succeeded",
            commitId: b.buildRun?.commitId ?? null,
            commitUrl: b.buildRun?.commitUrl ?? null,
            imageRef: b.image?.imageUrl ?? b.image?.ref ?? null,
            imageSha: b.image?.sha ?? null,
            createdAt: b.createdAt ?? null,
          },
        ];
      }),
    [data, buildSourceId],
  );
  return {
    builds,
    loading: loading && data === undefined,
    error,
    refetch: () => void refetch(),
  };
}
