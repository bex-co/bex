/**
 * The arithmetic behind Render's plan review (w3/m674,
 * docs/render-artifacts/postgres-dashboard.md § Plan review): what a Postgres
 * database bills a month once a compute or storage change applies.
 *
 * Render's `WP`: billed instances are the primary, the high-availability
 * standby and every read replica; the standby and each replica cost the plan's
 * price; storage is the size times the per-GB rate times every billed
 * instance. bex meters the same way (ADR030: an HA standby meters like the
 * primary; ADR009: each replica's instance and volume are metered like the
 * primary's), and its prices come from bex-api's price sheet through the plan
 * catalogue: `monthlyUsd` and `storageMonthlyUsdPerGB`. Amounts are whole
 * cents, so the lines always add up to the total they show.
 */

export interface DatabaseReviewInput {
  /** The plan's monthly price from the catalogue ("19.00"; "" when unlisted). */
  planMonthlyUsd: string;
  /** One GB of one instance's storage a month on the plan ("0.21"). */
  storageMonthlyUsdPerGB: string;
  /** High availability is on (the spec's intent, as the bill follows it). */
  highAvailability: boolean;
  readReplicas: number;
  /** The storage the change leaves the database with, or null for no line (a Free plan's is included). */
  storageGB: number | null;
}

export interface DatabaseReviewPricing {
  /** The primary, the standby when HA is on, and every read replica. */
  billedInstances: number;
  /** null when the catalogue lists no price for the plan. */
  planCents: number | null;
  /** The standby, priced like the primary; 0 without HA. */
  highAvailabilityCents: number | null;
  /** Every read replica at the plan's price. */
  readReplicasCents: number | null;
  /** null when there is no Storage line or no rate. */
  storageCents: number | null;
  /** The sum of the lines; null when a priced line is unknown. */
  totalCents: number | null;
}

/** An API decimal in `scale`ths of a dollar; null for "" or junk. */
function parseUsd(usd: string | undefined, scale: number): number | null {
  if (usd == null || !/^\d+(\.\d+)?$/.test(usd.trim())) return null;
  return Math.round(Number(usd) * scale);
}

/** Whole cents from an API decimal ("19.00" → 1900); null for "" or junk. */
export function usdToCents(usd: string | undefined): number | null {
  return parseUsd(usd, 100);
}

/** An API decimal with two places, the shape bex-api prices in ("945" → "9.45"). */
export function centsToUsd(cents: number): string {
  return (cents / 100).toFixed(2);
}

export function databaseReviewPricing({
  planMonthlyUsd,
  storageMonthlyUsdPerGB,
  highAvailability,
  readReplicas,
  storageGB,
}: DatabaseReviewInput): DatabaseReviewPricing {
  const billedInstances = 1 + (highAvailability ? 1 : 0) + readReplicas;
  const planCents = usdToCents(planMonthlyUsd);
  const highAvailabilityCents =
    planCents == null ? null : highAvailability ? planCents : 0;
  const readReplicasCents = planCents == null ? null : planCents * readReplicas;
  // The rate has three significant digits ("0.175" for a disk), so it is
  // carried in thousandths of a cent until the line is rounded once.
  const rate = parseUsd(storageMonthlyUsdPerGB, 100_000);
  const storageCents =
    storageGB == null || rate == null
      ? null
      : Math.round((storageGB * billedInstances * rate) / 1000);
  const storageMissing = storageGB != null && storageCents == null;
  const totalCents =
    planCents == null || storageMissing
      ? null
      : planCents +
        (highAvailabilityCents ?? 0) +
        (readReplicasCents ?? 0) +
        (storageCents ?? 0);
  return {
    billedInstances,
    planCents,
    highAvailabilityCents,
    readReplicasCents,
    storageCents,
    totalCents,
  };
}
