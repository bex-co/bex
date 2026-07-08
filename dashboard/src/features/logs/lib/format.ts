// formatLogTimestamp renders an RFC3339 log timestamp as Render's line clock —
// `03:36:01 AM`, local time. An empty or unparseable stamp renders blank rather
// than "Invalid Date", so a source that omitted the timestamp stays quiet.
export function formatLogTimestamp(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleTimeString(undefined, {
    hour12: true,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}
