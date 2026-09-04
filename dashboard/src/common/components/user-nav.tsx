import { useState } from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  LogOut,
  Loader2,
  Settings,
  Palette,
  Sun,
  Moon,
  Monitor,
  Globe,
  Check,
} from "lucide-react";
import { endBrowserSession } from "@/common/lib/ory/logout";
import { EMPTY_LOGIN_SEARCH } from "@/common/lib/auth/auth";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
} from "@/common/components/ui/dropdown-menu.tsx";
import { useTheme } from "@/common/providers/theme-provider";
import { useIsMobile } from "@/common/hooks/use-mobile";
import { useRootContext } from "@/common/hooks/use-root-context";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button.tsx";
import { Avatar, AvatarFallback } from "@/common/components/ui/avatar.tsx";
import {
  LANGUAGE_NAMES,
  SUPPORTED_LANGUAGES,
  persistLanguage,
  type SupportedLanguage,
} from "@/i18n";
import { switchLanguage } from "@/i18n/init";

interface UserAvatarButtonProps {
  userInitial: string;
  onClick?: () => void;
}

/**
 * Reusable user avatar button component
 * Displays a circular avatar button with user initial
 */
export function UserAvatarButton({
  userInitial,
  onClick,
  ...props
}: UserAvatarButtonProps) {
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      onClick={onClick}
      {...props}
      className="relative rounded-full hover:bg-accent p-0"
    >
      <Avatar className="h-8 w-8">
        <AvatarFallback>{userInitial}</AvatarFallback>
      </Avatar>
    </Button>
  );
}

const THEME_LABEL_KEYS = {
  light: "common.userMenuThemeLight",
  dark: "common.userMenuThemeDark",
  system: "common.userMenuThemeSystem",
} as const;

// Kratos identity traits are schema-defined per project (`traits: any`); bex's
// schema is `{ email, name? }` — name is the optional display-name trait
// (w4/m25, deploy/gitops/base/values/kratos.values.yaml).
type IdentityTraits = { email?: string; name?: string };

/**
 * User navigation dropdown — shows the signed-in user's avatar and provides
 * access to settings, theme, language, and logout. Session comes from Kratos (root
 * route's beforeLoad → sessions/whoami), not a GraphQL user query.
 */
export function UserNav() {
  const navigate = useNavigate();
  const router = useRouter();
  const { theme, setTheme } = useTheme();
  const isMobile = useIsMobile();
  const { session } = useRootContext();
  const { t, i18n } = useTranslations();
  const [signingOut, setSigningOut] = useState(false);

  const currentLanguage = i18n.language as SupportedLanguage;
  const handleLanguage = (lang: SupportedLanguage) => {
    persistLanguage(lang);
    // `switchLanguage` registers the lazy catalog before changing, so a
    // first-ever switch to a non-default language actually renders it instead of
    // the English fallback with `i18n.language` already moved (w6/m103 Bug A).
    void switchLanguage(lang);
  };

  const identity = session?.identity;
  const traits = identity?.traits as IdentityTraits | undefined;

  const handleLogout = () => {
    if (signingOut) return;
    setSigningOut(true);
    void (async () => {
      try {
        // The dropdown click is the explicit user action (codex #12). Auto-
        // logout on GET /auth/logout would still be CSRF, so that page keeps
        // its confirmation; this menu item does not need a second one.
        await endBrowserSession();
        await router.invalidate();
        void navigate({ to: "/auth/login", search: EMPTY_LOGIN_SEARCH });
      } catch (error) {
        console.error("Logout failed:", error);
        setSigningOut(false);
        toast.error(t("auth.logoutFailedSubtitle"));
      }
    })();
  };

  const handleSettings = () => {
    void navigate({ to: "/settings" });
  };

  const userInitial =
    (traits?.name || traits?.email)?.[0]?.toUpperCase() || "U";

  // On mobile, show a simple button that navigates to settings
  if (isMobile) {
    return (
      <UserAvatarButton userInitial={userInitial} onClick={handleSettings} />
    );
  }

  // On desktop, show the full dropdown menu
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <UserAvatarButton userInitial={userInitial} />
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-56" align="end" forceMount>
        <DropdownMenuLabel className="font-normal">
          <div className="flex flex-col space-y-1">
            <p className="text-sm font-medium leading-none">
              {traits?.name || "User"}
            </p>
            <p className="text-xs leading-none text-muted-foreground">
              {traits?.email || "—"}
            </p>
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={handleSettings}>
          <Settings className="mr-2 h-4 w-4" />
          <span>{t("common.userMenuSettings")}</span>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Palette className="mr-2 h-4 w-4" />
            <div className="flex flex-col">
              <span>{t("common.userMenuTheme")}</span>
              <span className="text-xs text-muted-foreground">
                {t(THEME_LABEL_KEYS[theme])}
              </span>
            </div>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuItem
              onClick={() => setTheme("light")}
              className={theme === "light" ? "bg-accent" : ""}
            >
              <Sun className="mr-2 h-4 w-4" />
              <span>{t("common.userMenuThemeLight")}</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => setTheme("dark")}
              className={theme === "dark" ? "bg-accent" : ""}
            >
              <Moon className="mr-2 h-4 w-4" />
              <span>{t("common.userMenuThemeDark")}</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => setTheme("system")}
              className={theme === "system" ? "bg-accent" : ""}
            >
              <Monitor className="mr-2 h-4 w-4" />
              <span>{t("common.userMenuThemeSystem")}</span>
            </DropdownMenuItem>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Globe className="mr-2 h-4 w-4" />
            <div className="flex flex-col">
              <span>{t("common.userMenuLanguage")}</span>
              <span className="text-xs text-muted-foreground">
                {LANGUAGE_NAMES[currentLanguage]}
              </span>
            </div>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            {SUPPORTED_LANGUAGES.map((lang) => (
              <DropdownMenuItem
                key={lang}
                onClick={() => handleLanguage(lang)}
                className={lang === currentLanguage ? "bg-accent" : ""}
              >
                <span className="flex-1">{LANGUAGE_NAMES[lang]}</span>
                {lang === currentLanguage && <Check className="h-4 w-4" />}
              </DropdownMenuItem>
            ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={handleLogout}
          variant="destructive"
          disabled={signingOut}
        >
          {signingOut ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <LogOut className="mr-2 h-4 w-4" />
          )}
          <span>
            {signingOut
              ? t("auth.loggingOutTitle")
              : t("common.userMenuLogOut")}
          </span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
