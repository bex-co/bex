// Keep this catalog in sync with Database.spec.version's CRD enum and the
// backend's guarded major-upgrade catalog. Descending order makes the newest
// supported version the create form's default. An in-place upgrade targets
// the newest only (bex-api's versionUpgradeAvailable, w3/m676).
export const POSTGRES_VERSIONS = ["18", "17", "16", "15", "14", "13"] as const;
