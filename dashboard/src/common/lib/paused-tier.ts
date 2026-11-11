/**
 * A catalog with each paused tier's reason in the dashboard's language
 * (w4/552). bex-api's reason is English only, and a tier is unavailable only
 * while paused (w2/m250), so the reason names the largest tier still offered,
 * as bex-api's does: catalogs list their tiers smallest-first. With no tier
 * left to suggest, bex-api's own reason stands.
 */
export function withPausedReasons<
  T extends { name: string; available?: boolean; unavailableReason?: string },
>(tiers: T[], reason: (name: string, largest: string) => string): T[] {
  let largest: string | undefined;
  for (let i = tiers.length - 1; i >= 0 && largest === undefined; i--) {
    if (tiers[i].available !== false) largest = tiers[i].name;
  }
  if (largest === undefined) return tiers;
  const suggest = largest;
  return tiers.map((tier) =>
    tier.available !== false
      ? tier
      : { ...tier, unavailableReason: reason(tier.name, suggest) },
  );
}
