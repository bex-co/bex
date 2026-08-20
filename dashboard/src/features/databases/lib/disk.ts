// Managed-Postgres disk sizing limits, mirrored from lego/types/tiers/tiers.yaml
// (the Go runtime source the operator and backend consume). Build-tested against
// the yaml in database-disk-autoscaling-control.test.tsx so a drift fails CI.

// The hard ceiling a Postgres volume can reach — Render's product/API cap — used
// both by the disk-autoscaling control and by the create dialog's disk field so
// neither offers a size the backend will reject.
export const DISK_AUTOSCALING_CAP_GB = 16 * 1024;
