/**
 * The cron job minimum in USD: what a cron job active in a calendar month is
 * charged at least (Render's $1 × 0.70, w9/m448). bex-api bills it from
 * `pricing.yaml`'s `cronJob.minimumUsdPerMonth`, and `cron-minimum.test.ts`
 * keeps this copy equal to it.
 */
export const CRON_MINIMUM_USD = "0.70";
