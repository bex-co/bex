import { useNavigate } from "@tanstack/react-router";
import { LogOut, Settings, Palette, Sun, Moon, Monitor } from "lucide-react";
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

// Kratos identity traits are schema-defined per project (`traits: any`); this
// dashboard's default schema is `{ email, name: { first, last } }`.
type IdentityTraits = { email?: string; name?: { first?: string } };

/**
 * User navigation dropdown — shows the signed-in user's avatar and provides
 * access to settings, theme, and logout. Session comes from Kratos (root
 * route's beforeLoad → sessions/whoami), not a GraphQL user query.
 */
export function UserNav() {
  const navigate = useNavigate();
  const { theme, setTheme } = useTheme();
  const isMobile = useIsMobile();
  const { session } = useRootContext();
  const { t } = useTranslations();

  const identity = session?.identity;
  const traits = identity?.traits as IdentityTraits | undefined;

  const handleLogout = () => {
    void navigate({ to: "/auth/logout" });
  };

  const handleSettings = () => {
    void navigate({ to: "/settings" });
  };

  const userInitial = traits?.email?.[0]?.toUpperCase() || "U";

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
              {traits?.name?.first || "User"}
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
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={handleLogout} variant="destructive">
          <LogOut className="mr-2 h-4 w-4" />
          <span>{t("common.userMenuLogOut")}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
