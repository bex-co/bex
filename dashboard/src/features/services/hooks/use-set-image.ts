import { SetImageDocument } from "@/graphql/definitions";
import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";

export interface UseSetImageResult {
  /**
   * Fires setImage; resolves true on success (toasted either way). An optional
   * registryCredentialId binds a private-registry pull credential to the image.
   */
  setImage: (
    id: string,
    image: string,
    registryCredentialId?: string,
  ) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the Settings Source control's "switch to a container image" path to
 * bex-api's `setImage` (w5/m76, Render's Update Source repo→image). The shared
 * source verb persists `spec.image` and clears the repo/branch source; the next
 * deploy uses the new image (no automatic deploy on change).
 */
export function useSetImage(): UseSetImageResult {
  const { run, busy } = useFieldMutation(
    SetImageDocument,
    (id: string, image: string, registryCredentialId?: string) => ({
      id,
      image,
      registryCredentialId: registryCredentialId ?? null,
    }),
    {
      success: "services.buildDeploySuccess",
      error: "services.buildDeployError",
    },
  );

  return { setImage: run, busy };
}
