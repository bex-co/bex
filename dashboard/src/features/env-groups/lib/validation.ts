import { isReservedEnvKey } from "@/features/services/lib/environment-draft";

/** bex-api and Render both accept any non-empty group display name. */
export function isValidEnvGroupName(name: string): boolean {
  return name.trim().length > 0;
}

/**
 * Matches Kubernetes/Render environment-variable key syntax, and rejects the
 * names bex owns. A group PORT is refused by bex-api (w2/m95 t003) — the
 * operator injects its own for every linked service — so the create dialog
 * must not offer it either.
 */
export function isValidEnvVarKey(key: string): boolean {
  const trimmed = key.trim();
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(trimmed) && !isReservedEnvKey(trimmed);
}

/** Matches the backend's Kubernetes Secret-key filename validation. */
export function isValidSecretFileName(name: string): boolean {
  const trimmed = name.trim();
  return (
    trimmed !== "" &&
    trimmed !== "." &&
    trimmed !== ".." &&
    /^[A-Za-z0-9_.-]+$/.test(trimmed)
  );
}
