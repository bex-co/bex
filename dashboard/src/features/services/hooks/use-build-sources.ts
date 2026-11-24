import { useMemo } from "react";
import { formatRepoLabel } from "@/features/services/lib/repo";
import {
  isNativeRuntime,
  RUNTIME_COMMANDS,
  type NativeRuntime,
} from "@/features/services/lib/runtime";
import { useBuildSourcesList } from "@/features/build-sources/hooks/use-build-sources-list";
import { useBuildSource as useBuildSourceDetail } from "@/features/build-sources/hooks/use-build-source";
import { useBuildSourceBuilds as useBuildSourceBuildsList } from "@/features/build-sources/hooks/use-build-source-builds";
import type {
  BuildSourceBuildView,
  BuildSourceView,
} from "@/features/build-sources/types";

export type { BuildSourceBuildView };

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

function toOption(b: BuildSourceView | null | undefined): BuildSourceOption | null {
  if (!b?.id) return null;
  const runtime = b.git?.runtime;
  return {
    id: b.id,
    name: b.name || b.id,
    source: b.git?.repoUrl ?? b.image?.imageUrl ?? "",
    kind: b.kind,
    branch: b.git?.branch || null,
    region: b.git?.region || null,
    nativeRuntime: b.git && isNativeRuntime(runtime) ? runtime : null,
  };
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
 * Reads the workspace's build sources for `ServiceSourcePicker` and Update Source
 * (w2/m254, unified with `features/build-sources` in w2/m613 t005 so all caches
 * stay in sync). `enabled` false skips the read until the dialog opens.
 */
export function useBuildSources(enabled: boolean) {
  const { buildSources, loading, error } = useBuildSourcesList({ enabled });
  const sources = useMemo(
    () =>
      buildSources.flatMap((b) => {
        const option = toOption(b);
        return option ? [option] : [];
      }),
    [buildSources],
  );
  return { sources, loading, error };
}

/** Reads the one build source a linked service builds from (null id skips). */
export function useBuildSource(id: string | null) {
  const { buildSource, loading } = useBuildSourceDetail(id);
  const source = useMemo(() => toOption(buildSource), [buildSource]);
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
  const { builds, loading, error, refetch } = useBuildSourceBuildsList(
    buildSourceId,
    enabled,
    50,
  );
  return {
    builds,
    loading,
    error,
    refetch: () => void refetch(),
  };
}
