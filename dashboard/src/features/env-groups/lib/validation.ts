import {
  isValidSecretFileName as isValidServiceSecretFileName,
  VALID_ENV_KEY,
} from "@/features/services/lib/environment-draft";

/** bex-api and Render both accept any non-empty group display name. */
export function isValidEnvGroupName(name: string): boolean {
  return name.trim().length > 0;
}

/** Matches Kubernetes/Render environment-variable key syntax. */
export function isValidEnvVarKey(key: string): boolean {
  return VALID_ENV_KEY.test(key.trim());
}

/** Matches the backend's Kubernetes Secret-key filename validation. */
export function isValidSecretFileName(name: string): boolean {
  return isValidServiceSecretFileName(name.trim());
}
