import { useId } from "react";
import { Card } from "@/common/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTheme } from "@/common/providers/theme-provider/hooks";
import {
  LOG_THEME_SETTINGS,
  THEMES,
  type LogThemeSetting,
} from "@/common/providers/theme-provider/settings";
import type { Theme } from "@/common/providers/theme-provider/type";
import {
  APPEARANCE_CARD_CLASS,
  APPEARANCE_SELECT_CLASS,
  AppearanceRow,
} from "./appearance-row";

/** One of the two theme rows' selects, named by its row. */
function AppearanceSelect<T extends string>({
  label,
  value,
  options,
  labels,
  onChange,
  ready,
}: {
  label: string;
  value: T;
  options: readonly T[];
  labels: Record<T, string>;
  onChange: (value: T) => void;
  ready: boolean;
}) {
  return (
    <Select
      value={ready ? value : ""}
      onValueChange={(next) => onChange(next as T)}
      disabled={!ready}
    >
      <SelectTrigger className={APPEARANCE_SELECT_CLASS} aria-label={label}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option} value={option}>
            {labels[option]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/**
 * Render's Appearance (w3/m528, w3/m531): Dashboard Theme (System (Default),
 * Light, Dark), Log Explorer Theme (Match Dashboard (Default), Light, Dark)
 * and High Contrast Mode, in Render's order. All three are saved on the
 * account, so every browser it signs in to opens in them; Dashboard Theme is
 * the setting the account menu's theme submenu changes, and keeps High
 * Contrast Mode as it is, as Render's does.
 */
export function AppearanceCard() {
  const { t } = useTranslations();
  const {
    theme,
    setTheme,
    highContrast,
    setHighContrast,
    logTheme,
    setLogTheme,
    ready,
  } = useTheme();
  const highContrastLabelId = useId();
  // The stored settings are read after hydration; until then the controls
  // show none, so the server's render and the first client render agree.
  const highContrastOn = ready && highContrast;
  return (
    <Card data-testid="appearance-card" className={APPEARANCE_CARD_CLASS}>
      <AppearanceRow label={t("auth.dashboardTheme")}>
        <AppearanceSelect<Theme>
          label={t("auth.dashboardTheme")}
          value={theme}
          options={THEMES}
          labels={{
            system: t("auth.dashboardThemeSystem"),
            light: t("common.userMenuThemeLight"),
            dark: t("common.userMenuThemeDark"),
          }}
          onChange={setTheme}
          ready={ready}
        />
      </AppearanceRow>
      <AppearanceRow label={t("auth.logExplorerTheme")}>
        <AppearanceSelect<LogThemeSetting>
          label={t("auth.logExplorerTheme")}
          value={logTheme}
          options={LOG_THEME_SETTINGS}
          labels={{
            MATCH_APP: t("auth.logExplorerThemeMatch"),
            ALWAYS_LIGHT: t("common.userMenuThemeLight"),
            ALWAYS_DARK: t("common.userMenuThemeDark"),
          }}
          onChange={setLogTheme}
          ready={ready}
        />
      </AppearanceRow>
      <AppearanceRow
        labelId={highContrastLabelId}
        label={t("auth.highContrastMode")}
        description={t("auth.highContrastModeDescription")}
      >
        <label className="flex items-center gap-2 text-sm">
          <Switch
            checked={highContrastOn}
            onCheckedChange={setHighContrast}
            disabled={!ready}
            aria-labelledby={highContrastLabelId}
          />
          {highContrastOn
            ? t("auth.highContrastEnabled")
            : t("auth.highContrastDisabled")}
        </label>
      </AppearanceRow>
    </Card>
  );
}
