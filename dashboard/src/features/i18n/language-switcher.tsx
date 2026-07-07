import { Check, Globe } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu.tsx";
import { Button } from "@/common/components/ui/button.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  LANGUAGE_NAMES,
  SUPPORTED_LANGUAGES,
  persistLanguage,
  type SupportedLanguage,
} from "@/i18n";

/**
 * Self-contained language switcher — no dependency on the dashboard sidebar,
 * so it also drops into unauthenticated chrome (e.g. auth-page-shell).
 */
export function LanguageSwitcher() {
  const { t, i18n } = useTranslations();
  const current = i18n.language as SupportedLanguage;

  const handleSelect = (lang: SupportedLanguage) => {
    void i18n.changeLanguage(lang);
    persistLanguage(lang);
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t("common.changeLanguage")}>
          <Globe className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {SUPPORTED_LANGUAGES.map((lang) => (
          <DropdownMenuItem key={lang} onClick={() => handleSelect(lang)}>
            <span className="flex-1">{LANGUAGE_NAMES[lang]}</span>
            {lang === current && <Check className="h-4 w-4" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
