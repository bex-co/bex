import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatDateTime, formatDateLong } from "@/common/lib/format";

/**
 * The string form of `LocalDateTime` (`common/components/local-time.tsx`), for
 * call sites that need the text itself rather than an element — a
 * `title=`/`aria-label` attribute, a value interpolated into a translated
 * sentence, or a per-row value inside a `.map()` where a component's own hook
 * can't be called. Returns `null` until hydration completes (so the caller can
 * omit or placeholder the text rather than emit the SSR container's UTC clock),
 * then the viewer-local string. See `use-is-hydrated.ts` for why the deferral
 * is necessary.
 */
export function useLocalDateTime(
  value: string | null | undefined,
): string | null {
  const hydrated = useIsHydrated();
  const { i18n } = useTranslations();
  return hydrated ? formatDateTime(value, i18n.language) : null;
}

/** The string form of `LocalDate` — see `useLocalDateTime`. */
export function useLocalDate(value: string | null | undefined): string | null {
  const hydrated = useIsHydrated();
  const { i18n } = useTranslations();
  return hydrated ? formatDateLong(value, i18n.language) : null;
}
