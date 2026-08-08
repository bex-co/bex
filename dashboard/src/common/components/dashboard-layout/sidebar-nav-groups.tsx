import { Link } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from "@/common/components/ui/sidebar.tsx";

export interface SidebarNavItem {
  labelKey: keyof typeof en;
  to: string;
  icon?: LucideIcon;
}

export interface SidebarNavGroup {
  /** Group heading; undefined = an unlabeled group (Render's top trio). */
  labelKey?: keyof typeof en;
  items: SidebarNavItem[];
}

/**
 * The one grouped-nav renderer both the global sidebar and the service
 * sidebar draw with (w1/m45) — labeled `SidebarGroup`s of menu links.
 * Consumers own what varies: the active predicate and any route params the
 * links need.
 */
export function SidebarNavGroups({
  groups,
  isItemActive,
  linkParams,
}: {
  groups: SidebarNavGroup[];
  isItemActive: (to: string) => boolean;
  linkParams?: Record<string, string>;
}) {
  const { t } = useTranslations();
  return (
    <>
      {groups.map((group, i) => (
        <SidebarGroup key={group.labelKey ?? i} className="py-1">
          {group.labelKey ? (
            <SidebarGroupLabel className="h-6">
              {t(group.labelKey)}
            </SidebarGroupLabel>
          ) : null}
          <SidebarGroupContent>
            <SidebarMenu className="gap-0.5">
              {group.items.map((item) => (
                <SidebarMenuItem key={item.to}>
                  <SidebarMenuButton
                    asChild
                    isActive={isItemActive(item.to)}
                    tooltip={t(item.labelKey)}
                  >
                    <Link to={item.to} params={linkParams}>
                      {item.icon ? <item.icon /> : null}
                      <span>{t(item.labelKey)}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      ))}
    </>
  );
}
