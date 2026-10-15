// Keep this catalog in sync with Database.spec.version's CRD enum and the
// backend's guarded major-upgrade catalog. Descending order makes the newest
// supported version the default upgrade target, matching Render's flow.
export const POSTGRES_VERSIONS = ["18", "17", "16", "15", "14", "13"] as const;

// CNPG's in-place major upgrade only targets 16 or newer; the backend refuses
// lower targets as POSTGRES_UPGRADE_TARGET_UNSUPPORTED (w4/308). Creation still
// offers the whole catalog.
const MIN_UPGRADE_TARGET = 16;

export function postgresUpgradeTargets(current: string | null): string[] {
  const major = Number(current);
  if (!Number.isInteger(major)) return [];
  return POSTGRES_VERSIONS.filter(
    (version) =>
      Number(version) > major && Number(version) >= MIN_UPGRADE_TARGET,
  );
}
