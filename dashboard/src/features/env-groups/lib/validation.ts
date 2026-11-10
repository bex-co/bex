import { isValidLabel } from "@/common/lib/utils/resource-label";
import {
  isValidSecretFileName as isValidServiceSecretFileName,
  VALID_ENV_KEY,
} from "@/features/services/lib/environment-draft";

/**
 * A group name is a free-form label: bex-api holds it to core.CheckLabel (no
 * control characters, at most 100 code points) since w4/419, so the form checks
 * the same rule instead of "any non-empty" (w21/036).
 */
export function isValidEnvGroupName(name: string): boolean {
  return isValidLabel(name);
}

/** Matches Kubernetes/Render environment-variable key syntax. */
export function isValidEnvVarKey(key: string): boolean {
  return VALID_ENV_KEY.test(key.trim());
}

/** Matches the backend's Kubernetes Secret-key filename validation. */
export function isValidSecretFileName(name: string): boolean {
  return isValidServiceSecretFileName(name.trim());
}
