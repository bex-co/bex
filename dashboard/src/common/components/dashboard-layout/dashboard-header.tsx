import { CircleHelp, Github, Library, SquareTerminal } from "lucide-react";
import { Authenticated } from "@/common/components/authenticated";
import { UserNav } from "@/common/components/user-nav.tsx";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { SidebarTrigger, useSidebar } from "@/common/components/ui/sidebar.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { DashboardBreadcrumbs } from "./dashboard-breadcrumbs";
import { GlobalSearch } from "./global-search";
import { NewResourceMenu } from "./new-resource-menu";

/**
 * Render-style global topbar. Its left side always answers “where am I?” and
 * its right side keeps workspace search and creation one click away. It lives
 * in DashboardLayout, so detail, settings, and creation routes all share the
 * same navigation instead of rendering an empty header.
 */
export function DashboardHeader() {
  const { state, isMobile, openMobile } = useSidebar();
  const showTrigger = isMobile ? !openMobile : state === "collapsed";

  return (
    <header className="sticky top-0 z-50 h-12 shrink-0 border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60">
      <div className="flex h-full min-w-0 items-center justify-between gap-2 px-3 sm:px-4">
        <div className="flex min-w-0 items-center gap-1">
          {showTrigger ? <SidebarTrigger className="mr-1 -ml-1" /> : null}
          <DashboardBreadcrumbs />
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <GlobalSearch />
          <NewResourceMenu />
          <HelpMenu />
          <Authenticated>
            <UserNav />
          </Authenticated>
        </div>
      </div>
    </header>
  );
}

export function HelpMenu() {
  const { t } = useTranslations();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="icon-sm"
          className="hidden sm:inline-flex"
          aria-label={t("common.topbarHelp")}
        >
          <CircleHelp />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem asChild>
          <a href="https://bex.co/docs" target="_blank" rel="noreferrer">
            <Library />
            {t("common.topbarDocumentation")}
          </a>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a href="https://bex.co/docs/cli" target="_blank" rel="noreferrer">
            <SquareTerminal />
            {t("common.topbarCliGuide")}
          </a>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a
            href="https://github.com/bex-co/bex"
            target="_blank"
            rel="noreferrer"
          >
            <Github />
            {t("common.topbarRepository")}
          </a>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
