import { LinkBuildSourceDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

/**
 * Links a service to a build source (Render's Update Source → build source,
 * w2/m254): the service's own repo or image is cleared and it builds from the
 * source from its next deploy; nothing deploys now. startCommand is required
 * for a native-runtime source and refused for a Docker or image one.
 */
export function useLinkBuildSource() {
  const { run, busy, lastError } = useFieldMutation(
    LinkBuildSourceDocument,
    (serviceId: string, buildSourceId: string, startCommand?: string) => ({
      serviceId,
      buildSourceId,
      startCommand: startCommand ?? null,
    }),
    {
      success: "services.sourceUpdateSuccess",
      error: "services.sourceUpdateError",
    },
  );
  return { linkBuildSource: run, busy, lastError };
}
