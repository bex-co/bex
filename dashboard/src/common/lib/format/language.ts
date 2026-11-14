import { zhCN } from "date-fns/locale/zh-CN";

/** Whether an i18n language ("zh", "zh-CN") is the dashboard's Chinese. */
export function isChinese(language: string | undefined): boolean {
  return !!language?.toLowerCase().startsWith("zh");
}

/** The date-fns locale for a dashboard language; undefined is date-fns' en-US. */
export function dateFnsLocale(language: string | undefined) {
  return isChinese(language) ? zhCN : undefined;
}

/**
 * One cached Intl.DateTimeFormat per dashboard language for `options`, built
 * once at module load. zh gets a 12-hour clock (上午/下午) wherever the en
 * reading has AM/PM, which zh-CN's own default would render 24-hour.
 */
export function perLanguage(options: Intl.DateTimeFormatOptions) {
  const en = new Intl.DateTimeFormat("en-US", options);
  const zh = new Intl.DateTimeFormat("zh-CN", {
    ...options,
    ...(options.hour || options.timeStyle ? { hour12: true } : {}),
  });
  return (language: string | undefined) => (isChinese(language) ? zh : en);
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
