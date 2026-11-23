import { SetImageDocument } from "@/graphql/definitions";
import {
  useSourceMutation,
  type SourceMutationResult,
} from "@/features/services/hooks/use-source-mutation";

export interface UseSetImageResult {
  /**
   * Fires setImage; resolves `{ ok, deployId }` (toasted either way). An
   * optional registryCredentialId binds a private-registry pull credential to
   * the image.
   */
  setImage: (
    id: string,
    image: string,
    registryCredentialId?: string,
    deploy?: boolean,
  ) => Promise<SourceMutationResult>;
  busy: boolean;
  /** The server's refusal from the last failed setImage, else null. */
  lastError: string | null;
}

/**
 * Wires Settings → Update Source's Existing Image path to bex-api's `setImage`
 * (w5/m76, w4/m593). Persists `spec.image`, clears the repo/branch source, and
 * when `deploy` is true (default) immediately triggers one deploy.
 */
export function useSetImage(): UseSetImageResult {
  const { run, busy, lastError } = useSourceMutation(
    SetImageDocument,
    (
      id: string,
      image: string,
      registryCredentialId?: string,
      deploy?: boolean,
    ) => ({
      id,
      image,
      registryCredentialId: registryCredentialId ?? null,
      deploy: deploy ?? true,
    }),
    (data) => data?.setImage?.latestDeployId ?? null,
  );

  return { setImage: run, busy, lastError };
}
