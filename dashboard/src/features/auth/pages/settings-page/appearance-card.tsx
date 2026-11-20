import { Palette } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTheme } from "@/common/providers/theme-provider/hooks";
import type { Theme } from "@/common/providers/theme-provider/type";

/**
 * Render's Appearance ▸ Dashboard Theme (w3/m528): System (the default),
 * Light or Dark, the same setting as the account menu's theme submenu, so a
 * change in either shows in the other. Like that menu, it is kept in this
 * browser, not on the account (Render stores it server-side).
 */
export function AppearanceCard() {
  const { t } = useTranslations();
  const { theme, setTheme } = useTheme();
  // The browser's stored theme is read after hydration; until then the
  // select shows none, so the server's render and the first client render
  // agree.
  const hydrated = useIsHydrated();
  const labels: Record<Theme, string> = {
    system: t("auth.dashboardThemeSystem"),
    light: t("common.userMenuThemeLight"),
    dark: t("common.userMenuThemeDark"),
  };
  return (
    <Card data-testid="appearance-card">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Palette className="size-4" />
          {t("auth.dashboardTheme")}
        </CardTitle>
        <CardDescription>{t("auth.dashboardThemeDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <Select
          value={hydrated ? theme : ""}
          onValueChange={(value) => setTheme(value as Theme)}
          disabled={!hydrated}
        >
          <SelectTrigger className="w-56" aria-label={t("auth.dashboardTheme")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {(["system", "light", "dark"] as const).map((value) => (
              <SelectItem key={value} value={value}>
                {labels[value]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </CardContent>
    </Card>
  );
}
