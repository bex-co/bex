import type { ImageCheck } from "@/features/services/hooks/use-image-check";
import { UndeployableImageFailureCode } from "@/graphql/schema-types";
import { dockerImageProblem } from "@/common/lib/utils/docker-image";

/**
 * A stand-in for useImageCheck in page tests that render without Apollo: an
 * image answers at once, deployable unless it is a private image (a path
 * with "private" in it) checked with no credential, which reads as Render's
 * does_not_exist. A path with "slow" in it is still being checked. The hook's
 * own tests cover the registry read.
 */
export function fakeUseImageCheck({
  image,
  credentialId,
  enabled,
}: {
  image: string;
  credentialId: string;
  enabled: boolean;
}): ImageCheck {
  const value = image.trim();
  const idle = !enabled || value === "" || dockerImageProblem(value) !== null;
  const missing = value.includes("private") && credentialId === "";
  return {
    status: idle
      ? "idle"
      : value.includes("slow")
        ? "checking"
        : missing
          ? "failed"
          : "deployable",
    failureCode:
      !idle && missing ? UndeployableImageFailureCode.DoesNotExist : null,
    withCredential: credentialId !== "",
    notePaste: () => {},
  };
}
