import { parseISO, isValid, format } from "date-fns";
import { dateFnsLocale, isChinese } from "./language";

/**
 * Format a timestamp as the dashboard's standard full date + time, local time:
 * "July 16, 2026 at 12:57 AM", or "2026年7月16日 上午12:57" in the zh
 * dashboard. One fixed pattern per dashboard language (never the browser's
 * locale), so the calendar text is identical everywhere within a language.
 *
 * The *clock reading*, though, follows the runtime's own timezone — UTC in the
 * SSR container, the viewer's zone in the browser — so calling this in a render
 * body bakes a wrong (UTC) time into the SSR markup that hydration then freezes
 * on screen (w6/030 → w6/m107). Render it through `LocalDateTime` /
 * `useLocalDateTime` (`common/components/local-time.tsx`), which defers it to a
 * post-hydration client render; call this directly only off the render path.
 * @param dateString - RFC3339/ISO timestamp to format
 * @param language - the active i18n language ("en", "zh")
 * @returns Formatted date-time or null if missing/invalid
 */
export const formatDateTime = (
  dateString: string | null | undefined,
  language?: string,
): string | null => {
  const date = parseDate(dateString);
  if (!date) return null;
  return format(
    date,
    isChinese(language) ? "yyyy年M月d日 ah:mm" : "MMMM d, yyyy 'at' h:mm a",
    { locale: dateFnsLocale(language) },
  );
};

/**
 * Format a timestamp as the date-only counterpart of formatDateTime, local
 * time: "July 16, 2026" ("2026年7月16日" in zh). For fields where the clock
 * reading carries no meaning (creation dates, expiry dates).
 * @param dateString - RFC3339/ISO timestamp to format
 * @param language - the active i18n language ("en", "zh")
 * @returns Formatted date or null if missing/invalid
 */
export const formatDateLong = (
  dateString: string | null | undefined,
  language?: string,
): string | null => {
  const date = parseDate(dateString);
  if (!date) return null;
  return format(date, isChinese(language) ? "yyyy年M月d日" : "MMMM d, yyyy");
};

const parseDate = (dateString: string | null | undefined): Date | null => {
  if (!dateString) return null;
  const date = parseISO(dateString);
  return isValid(date) ? date : null;
};
