// Presentation for backend enum tokens ("web_service", "update_in_progress")
// that have no translation entry; names and ids must not go through this.
export function humanizeToken(value: string): string {
  const spaced = value.replace(/_+/g, " ").trim();
  if (!spaced) return value;
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

// Intl.DateTimeFormat construction is the expensive part (locale-data
// resolution); the app only ever uses a couple of locales, so cache them.
const timestampFormatters = new Map<string, Intl.DateTimeFormat>();

export function formatTimestamp(
  value: string | number | Date,
  locale = "en",
): string {
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return "—";
  let formatter = timestampFormatters.get(locale);
  if (!formatter) {
    formatter = new Intl.DateTimeFormat(locale, {
      dateStyle: "medium",
      timeStyle: "short",
    });
    timestampFormatters.set(locale, formatter);
  }
  return formatter.format(date);
}
