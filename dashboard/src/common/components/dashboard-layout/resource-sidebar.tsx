import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import { ChevronLeft } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
} from "@/common/components/ui/sidebar.tsx";
import { SidebarBrand } from "./sidebar-brand";

/**
 * The frame of a resource's own sidebar, a service's or a Postgres
 * database's (w1/m45, w3/m641): the logo and workspace switcher pinned on top,
 * a back link to the workspace overview, the resource's name, then its
 * navigation.
 */
export function ResourceSidebar({
  name,
  children,
}: {
  name: string;
  children: ReactNode;
}) {
  const { t } = useTranslations();
  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarBrand />
      </SidebarHeader>
      <SidebarContent>
        <div className="flex flex-col gap-2 px-2 pt-2 pb-1 group-data-[collapsible=icon]:hidden">
          <Link
            to="/"
            className="relative flex items-center gap-1 text-xs font-medium text-muted-foreground after:absolute after:inset-x-0 after:-inset-y-1 after:content-[''] hover:text-foreground"
          >
            <ChevronLeft className="size-3.5" />
            {t("common.navBackToDashboard")}
          </Link>
          <p className="truncate text-sm font-semibold">{name}</p>
        </div>
        {children}
      </SidebarContent>
    </Sidebar>
  );
}
