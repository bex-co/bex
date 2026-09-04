import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { en } from "@/i18n";

/**
 * Type-safe wrapper around react-i18next's `useTranslation`.
 *
 * In dev, warns on keys that skip the `namespace.` prefix convention and on
 * keys missing from the `en` resources (the source of truth for what keys
 * exist), since a typo'd key otherwise silently renders itself as the value.
 *
 * Pluralization convention (w6/062): a count-bearing string is authored as
 * `"ns.key_one"` + `"ns.key_other"` in `en` (zh gets only `_other` — Chinese
 * has a single plural category) and called as `t("ns.key", { count })` with a
 * NUMERIC count; i18next resolves the suffix natively. The base key never
 * exists in the catalog, so the dev guard accepts it when its `_other` variant
 * does — and warns when such a key is called without a numeric `count`.
 */
export function useTranslations() {
  const { t: i18nT, i18n } = useTranslation();

  const t = useCallback(
    (
      key: keyof typeof en,
      params?: Record<string, string | number>,
    ): string => {
      if (import.meta.env.DEV) {
        if (!key.includes(".")) {
          console.error(
            `Translation key "${key}" is missing a namespace prefix. ` +
              `Use t("namespace.keyName") — e.g. t("common.appName").`,
          );
        }
        if (!(key in en)) {
          if (`${key}_other` in en) {
            // A native-plural key: only its `_one`/`_other` variants exist.
            if (typeof params?.count !== "number") {
              console.warn(
                `Translation key "${key}" is pluralized (_one/_other) and ` +
                  `needs a numeric \`count\` param to resolve.`,
              );
            }
          } else {
            console.warn(
              `Translation key "${key}" was not found in the en resources.`,
            );
          }
        }
      }

      return i18nT(key, params);
    },
    [i18nT],
  );

  return { t, i18n };
}
