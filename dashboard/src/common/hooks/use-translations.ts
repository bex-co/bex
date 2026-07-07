import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { en } from "@/i18n";

/**
 * Type-safe wrapper around react-i18next's `useTranslation`.
 *
 * In dev, warns on keys that skip the `namespace.` prefix convention and on
 * keys missing from the `en` resources (the source of truth for what keys
 * exist), since a typo'd key otherwise silently renders itself as the value.
 */
export function useTranslations() {
  const { t: i18nT, i18n } = useTranslation();

  const t = useCallback(
    (key: keyof typeof en, params?: Record<string, string | number>): string => {
      if (import.meta.env.DEV) {
        if (!key.includes(".")) {
          console.error(
            `Translation key "${key}" is missing a namespace prefix. ` +
              `Use t("namespace.keyName") — e.g. t("common.appName").`,
          );
        }
        if (!(key in en)) {
          console.warn(`Translation key "${key}" was not found in the en resources.`);
        }
      }

      return i18nT(key, params);
    },
    [i18nT],
  );

  return { t, i18n };
}
