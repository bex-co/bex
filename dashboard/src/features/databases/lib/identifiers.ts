/** Matches the backend/CRD's safe unquoted PostgreSQL identifier contract. */
export function isValidPostgresIdentifier(value: string): boolean {
  return /^[a-z_][a-z0-9_]{0,62}$/.test(value);
}

/** Database names PostgreSQL itself owns (mirrors ReservedPostgresDatabaseName). */
export function isReservedPostgresDatabaseName(value: string): boolean {
  return value === "postgres" || value === "template0" || value === "template1";
}

/** Roles bex must never manage (mirrors ReservedPostgresRole): the superuser,
 *  CNPG's replication role, and PostgreSQL's reserved pg_ prefix. */
export function isReservedPostgresRole(value: string): boolean {
  return (
    value === "postgres" ||
    value === "streaming_replica" ||
    value.startsWith("pg_")
  );
}
