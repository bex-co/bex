import { LinkBuildSourceDocument } from "@/graphql/definitions";
import {
  useSourceMutation,
  type SourceMutationResult,
} from "@/features/services/hooks/use-source-mutation";

export interface UseLinkBuildSourceResult {
  linkBuildSource: (
    serviceId: string,
    buildSourceId: string,
    startCommand?: string,
    deploy?: boolean,
  ) => Promise<SourceMutationResult>;
  busy: boolean;
  lastError: string | null;
}

/**
 * Links a service to a build source (Render's Update Source → build source,
 * w2/m254, w4/m593): clears the service's own repo/image and, when `deploy` is
 * true (default), triggers one deploy when the build source already has a
 * succeeded build (otherwise saves cleanly and waits for the first build).
 */
export function useLinkBuildSource(): UseLinkBuildSourceResult {
  const { run, busy, lastError } = useSourceMutation(
    LinkBuildSourceDocument,
    (
      serviceId: string,
      buildSourceId: string,
      startCommand?: string,
      deploy?: boolean,
    ) => ({
      serviceId,
      buildSourceId,
      startCommand: startCommand ?? null,
      deploy: deploy ?? true,
    }),
    (data) => data?.linkBuildSource?.latestDeployId ?? null,
    { withConfirm: false },
  );
  return { linkBuildSource: run, busy, lastError };
}
