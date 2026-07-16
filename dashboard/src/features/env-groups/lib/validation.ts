/** bex-api and Render both accept any non-empty group display name. */
export function isValidEnvGroupName(name: string): boolean {
  return name.trim().length > 0;
}

/** Matches Kubernetes/Render environment-variable key syntax. */
export function isValidEnvVarKey(key: string): boolean {
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(key.trim());
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
