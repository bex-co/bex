import { SetRepoDocument } from "@/graphql/definitions";
import {
  useSourceMutation,
  type SourceMutationResult,
} from "@/features/services/hooks/use-source-mutation";

export interface UseSetRepoResult {
  /** Fires setRepo; resolves `{ ok, deployId }` (toasted either way). */
  setRepo: (
    id: string,
    source: RepoSourceUpdate,
  ) => Promise<SourceMutationResult>;
  busy: boolean;
  lastError: string | null;
}

export interface RepoSourceUpdate {
  repo: string;
  branch?: string;
  runtime?: string;
  buildCommand?: string;
  startCommand?: string;
  command?: string;
  publishPath?: string;
  deploy?: boolean;
}

/**
 * Wires Settings → Update Source's Git path to bex-api's `setRepo` (w5/m54,
 * w5/m76, w4/m593). Sends confirmed build settings atomically and, when
 * `deploy: true`, triggers one deploy and returns its id for navigation.
 */
export function useSetRepo(): UseSetRepoResult {
  const { run, busy, lastError } = useSourceMutation(
    SetRepoDocument,
    (
      id: string,
      {
        repo,
        branch,
        runtime,
        buildCommand,
        startCommand,
        command,
        publishPath,
        deploy,
      }: RepoSourceUpdate,
    ) => ({
      id,
      repo,
      branch: branch ?? null,
      runtime: runtime ?? null,
      buildCommand: buildCommand ?? null,
      startCommand: startCommand ?? null,
      command: command ?? null,
      publishPath: publishPath ?? null,
      deploy: deploy ?? true,
    }),
    (data) => data?.setRepo?.latestDeployId ?? null,
  );

  return { setRepo: run, busy, lastError };
}
