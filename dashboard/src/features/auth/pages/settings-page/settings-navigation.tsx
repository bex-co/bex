import { Blocks, KeyRound, ShieldCheck, UserRound } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";

export function SettingsNavigation({ className }: { className?: string }) {
  const { t } = useTranslations();
  const items = [
    {
      href: "#account",
      label: t("auth.accountSection"),
      icon: UserRound,
    },
    {
      href: "#integrations",
      label: t("auth.integrationsSection"),
      icon: Blocks,
    },
    {
      href: "#access",
      label: t("auth.accessSection"),
      icon: KeyRound,
    },
    {
      href: "#security",
      label: t("auth.securityComplianceSection"),
      icon: ShieldCheck,
    },
  ];

  return (
    <nav
      aria-label={t("auth.settingsNavigation")}
      className={cn("min-w-0", className)}
    >
      <div className="flex gap-1 overflow-x-auto lg:flex-col lg:overflow-visible">
        {items.map(({ href, label, icon: Icon }) => (
          <a
            key={href}
            href={href}
            className="flex shrink-0 items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
          >
            <Icon aria-hidden="true" className="size-4" />
            {label}
          </a>
        ))}
      </div>
    </nav>
  );
}
