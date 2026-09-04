import { useQuery } from "@apollo/client/react";
import { useDebounce } from "@/common/hooks/use-debounce";
import { RepoRuntimeDetectionDocument } from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { RUNTIME_DEFS, type GitRuntime } from "@/features/services/lib/runtime";

const ROOT_DIR_DEBOUNCE_MS = 400;
const RUNTIMES = new Set<GitRuntime>(RUNTIME_DEFS.map(({ id }) => id));

function gitRuntime(value: string | null | undefined): GitRuntime | null {
  return value && RUNTIMES.has(value as GitRuntime)
    ? (value as GitRuntime)
    : null;
}

/**
 * Best-effort runtime inference for one connected GitHub repo directory.
 *
 * Repo and branch changes query immediately; only Root Directory keystrokes are
 * debounced. While an input is unsettled or a request is in flight, the hook
 * returns no verdict so stale Apollo data can never rewrite the form. Expected
 * backend and transport failures are intentionally silent.
 */
export function useRepoRuntimeDetection({
  repo,
  branch,
  rootDir,
}: {
  repo: string | null;
  branch: string;
  rootDir: string;
}) {
  const { currentWorkspaceId } = useWorkspace();
  const debouncedRootDir = useDebounce(rootDir, ROOT_DIR_DEBOUNCE_MS);
  const skip = !repo || !branch || currentWorkspaceId == null;
  const { data, loading } = useQuery(RepoRuntimeDetectionDocument, {
    variables: {
      repo: repo ?? "",
      branch,
      rootDir: debouncedRootDir || null,
      ownerId: currentWorkspaceId,
    },
    skip,
    fetchPolicy: "network-only",
    errorPolicy: "all",
  });
  const settled = debouncedRootDir === rootDir;
  return !skip && settled && !loading
    ? gitRuntime(data?.repoRuntimeDetection?.runtime)
    : null;
}
