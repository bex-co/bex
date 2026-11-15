import type { FormatDistanceToken, Locale } from "date-fns";
import { zhCN } from "date-fns/locale/zh-CN";
import {
  asSupportedLanguage,
  DEFAULT_LANGUAGE,
  LANGUAGE_TAGS,
  type SupportedLanguage,
} from "@/i18n/config";

/**
 * The dashboard language an i18n language string names — "zh", "zh-CN" and
 * "pt-BR" resolve to their base language; anything unsupported reads as en.
 */
export function dashboardLanguage(
  language: string | undefined,
): SupportedLanguage {
  return (
    asSupportedLanguage(language?.toLowerCase().split("-")[0]) ??
    DEFAULT_LANGUAGE
  );
}

// date-fns' distance tokens and the Intl.RelativeTimeFormat unit each words.
const DISTANCE_UNITS: Partial<
  Record<FormatDistanceToken, Intl.RelativeTimeFormatUnit>
> = {
  xSeconds: "second",
  xMinutes: "minute",
  xHours: "hour",
  xDays: "day",
  xWeeks: "week",
  xMonths: "month",
  xYears: "year",
};

/**
 * A date-fns locale that only words a `formatDistanceStrict` result: date-fns
 * still picks the unit and rounds ("2 hours", DST-normalized), and the
 * runtime's own CLDR data says it in the language (`vor 2 Stunden`,
 * `2 時間前`). A full date-fns locale is ~2.5 KB gzip apiece in the entry
 * chunk; this costs nothing. It carries no month names or patterns, so it must
 * never reach `format()` — its type admits only `formatDistance` callers, and
 * `format-date-time.ts` uses Intl for these languages.
 */
type DistanceLocale = Pick<Locale, "code" | "formatDistance">;

function intlDistanceLocale(tag: string): DistanceLocale {
  const relative = new Intl.RelativeTimeFormat(tag, { numeric: "always" });
  return {
    code: tag,
    formatDistance: (token, count, options) => {
      const unit = DISTANCE_UNITS[token] ?? "second";
      if (!options?.addSuffix) {
        // A bare amount with no direction: "2 Stunden", "2 時間".
        return new Intl.NumberFormat(tag, {
          style: "unit",
          unit,
          unitDisplay: "long",
        }).format(count);
      }
      // `comparison` > 0 is a future instant ("in 2 hours").
      const future = (options.comparison ?? -1) > 0;
      return relative.format(future ? count : -count, unit);
    },
  };
}

const distanceLocales = new Map<SupportedLanguage, DistanceLocale>();

/**
 * The date-fns locale `formatDistanceStrict` words a distance with: undefined
 * is date-fns' en-US; zh keeps date-fns' own zh-CN wording; every other
 * language words it through Intl (see `intlDistanceLocale`).
 */
export function dateFnsLocale(
  language: string | undefined,
): DistanceLocale | undefined {
  const lang = dashboardLanguage(language);
  if (lang === "en") return undefined;
  if (lang === "zh") return zhCN;
  let locale = distanceLocales.get(lang);
  if (!locale) {
    locale = intlDistanceLocale(LANGUAGE_TAGS[lang]);
    distanceLocales.set(lang, locale);
  }
  return locale;
}

/**
 * One cached Intl.DateTimeFormat per dashboard language for `options`, built
 * on first use. en reads exactly `options`; zh gets a 12-hour clock (上午/下午)
 * wherever the en reading has AM/PM, which zh-CN's own default would render
 * 24-hour. Every other language reads its own clock convention — 24-hour in
 * ja/es/pt/ru/de, 오전/오후 in ko — since en's explicit `hour12` is Render's
 * English reading, not theirs.
 */
export function perLanguage(options: Intl.DateTimeFormatOptions) {
  const formats = new Map<SupportedLanguage, Intl.DateTimeFormat>();
  return (language: string | undefined) => {
    const lang = dashboardLanguage(language);
    let format = formats.get(lang);
    if (!format) {
      format = new Intl.DateTimeFormat(
        LANGUAGE_TAGS[lang],
        languageOptions(lang, options),
      );
      formats.set(lang, format);
    }
    return format;
  };
}

function languageOptions(
  lang: SupportedLanguage,
  options: Intl.DateTimeFormatOptions,
): Intl.DateTimeFormatOptions {
  if (lang === "en") return options;
  const hasClock = Boolean(options.hour || options.timeStyle);
  if (lang === "zh") return hasClock ? { ...options, hour12: true } : options;
  const native = { ...options };
  delete native.hour12;
  return native;
}

/** Whether the runtime knows `zone` as an IANA time zone. */
export function isValidTimeZone(zone: string): boolean {
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: zone });
    return true;
  } catch {
    return false;
  }
}
