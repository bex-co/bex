/** Matches the backend/CRD's safe unquoted PostgreSQL identifier contract. */
export function isValidPostgresIdentifier(value: string): boolean {
  return /^[a-z_][a-z0-9_]{0,62}$/.test(value);
}
