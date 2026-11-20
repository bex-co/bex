import { useEffect } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  UpdateUserLogThemeSettingDocument,
  UpdateUserThemeSettingDocument,
  UserThemeSettingsDocument,
  type LogThemeSetting as LogThemeSettingValue,
  type ThemeSetting as ThemeSettingValue,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useTheme } from "@/common/providers/theme-provider/hooks";
import {
  isLogThemeSetting,
  isThemeSetting,
  type ThemeSettings,
} from "@/common/providers/theme-provider/settings";

/**
 * Binds the signed-in account to the theme provider (w3/m531): reads the
 * account's `user { id themeSetting logThemeSetting }`, which then wins over
 * the browser's copy, and writes each change with Render's
 * `updateUserThemeSetting` and `updateUserLogThemeSetting`. A write the server
 * refuses says so in a toast and rejects, so the provider puts the previous
 * setting back.
 */
export function useAccountThemeSync() {
  const { bindAccount } = useTheme();
  const { t } = useTranslations();
  const { data } = useQuery(UserThemeSettingsDocument, {
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const [updateTheme] = useMutation(UpdateUserThemeSettingDocument);
  const [updateLogTheme] = useMutation(UpdateUserLogThemeSettingDocument);
  const id = data?.user?.id;
  const themeSetting = data?.user?.themeSetting;
  const logThemeSetting = data?.user?.logThemeSetting;

  useEffect(() => {
    if (
      !id ||
      !isThemeSetting(themeSetting) ||
      !isLogThemeSetting(logThemeSetting)
    )
      return;
    bindAccount({
      settings: { themeSetting, logThemeSetting },
      write: async (patch) => {
        const saved: Partial<ThemeSettings> = {};
        try {
          if (patch.themeSetting) {
            const { data: written } = await updateTheme({
              variables: {
                id,
                themeSetting: patch.themeSetting as ThemeSettingValue,
              },
            });
            const value = written?.updateUserThemeSetting?.themeSetting;
            if (!isThemeSetting(value)) throw new Error("theme not saved");
            saved.themeSetting = value;
          }
          if (patch.logThemeSetting) {
            const { data: written } = await updateLogTheme({
              variables: {
                id,
                logThemeSetting: patch.logThemeSetting as LogThemeSettingValue,
              },
            });
            const value = written?.updateUserLogThemeSetting?.logThemeSetting;
            if (!isLogThemeSetting(value)) throw new Error("theme not saved");
            saved.logThemeSetting = value;
          }
        } catch (cause) {
          toast.error(mutationErrorMessage(cause, t("common.themeNotSaved")));
          throw cause;
        }
        return saved;
      },
    });
  }, [
    id,
    themeSetting,
    logThemeSetting,
    bindAccount,
    updateTheme,
    updateLogTheme,
    t,
  ]);

  // Signing out, or leaving the signed-in shell, unbinds the account.
  useEffect(() => () => bindAccount(null), [bindAccount]);
}
