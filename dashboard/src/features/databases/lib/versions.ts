// Keep this catalog in sync with Database.spec.version's CRD enum and the
// backend's guarded major-upgrade catalog. Descending order makes the newest
// supported version the default upgrade target, matching Render's flow.
export const POSTGRES_VERSIONS = ["18", "17", "16", "15", "14", "13"] as const;

export function postgresUpgradeTargets(current: string | null): string[] {
  const major = Number(current);
  if (!Number.isInteger(major)) return [];
  return POSTGRES_VERSIONS.filter((version) => Number(version) > major);
}
