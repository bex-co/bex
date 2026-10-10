import type { Translate } from "@/common/hooks/use-translations";
import i18n from "@/i18n/init";

/**
 * The app's real translator fixed to one language — the same catalogs and
 * single-brace interpolation `useTranslations()` resolves through — for pure
 * helpers that take the caller's `t` (w4/m188).
 */
export function translator(lng: "en" | "zh"): Translate {
  return i18n.getFixedT(lng) as unknown as Translate;
}
