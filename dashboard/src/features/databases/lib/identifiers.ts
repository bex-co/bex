/** Matches the backend/CRD's safe unquoted PostgreSQL identifier contract. */
export function isValidPostgresIdentifier(value: string): boolean {
  return /^[a-z_][a-z0-9_]{0,62}$/.test(value);
}

/** Database names PostgreSQL itself owns (mirrors ReservedPostgresDatabaseName). */
export function isReservedPostgresDatabaseName(value: string): boolean {
  return value === "postgres" || value === "template0" || value === "template1";
}

/** Roles bex must never manage (mirrors ReservedPostgresRole): those CNPG
 *  reserves (postgres, streaming_replica, the cnpg_ and pg_ prefixes) and
 *  those PostgreSQL refuses to create (public, none). */
export function isReservedPostgresRole(value: string): boolean {
  return (
    ["postgres", "streaming_replica", "public", "none"].includes(value) ||
    value.startsWith("pg_") ||
    value.startsWith("cnpg_")
  );
}
