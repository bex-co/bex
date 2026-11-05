/**
 * Format an amount given in the currency's minor unit (Stripe's integer
 * amounts: cents for USD, whole yen for JPY) for `locale`, e.g. 300 usd →
 * "$3.00". The currency's own fraction digits decide the scale. The narrow
 * symbol keeps "$" in every locale, so a caller that appends the ISO code does
 * not read "US$3.00 USD". An unknown currency code falls back to
 * "3.00 XYZ" rather than throwing.
 */
export function formatMinorUnits(
  amount: number,
  currency: string,
  locale: string,
): string {
  const code = currency.toUpperCase();
  try {
    const format = new Intl.NumberFormat(locale, {
      style: "currency",
      currency: code,
      currencyDisplay: "narrowSymbol",
    });
    const digits = format.resolvedOptions().maximumFractionDigits ?? 2;
    return format.format(amount / 10 ** digits);
  } catch {
    return `${(amount / 100).toFixed(2)} ${code}`;
  }
}
