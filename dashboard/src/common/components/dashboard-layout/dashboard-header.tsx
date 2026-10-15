import { useEffect, useRef } from "react";
import { useRouterState } from "@tanstack/react-router";
import { CircleHelp, Library, SquareTerminal } from "lucide-react";
import { GithubIcon } from "@/common/components/github-icon";
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
  const { isMobile, openMobile, setOpenMobile } = useSidebar();
  // The shell persists across routes, so nothing unmounts the mobile drawer
  // when one of its links navigates: close it once a different destination
  // commits (w4/283). Mounting, a modified click that opens another tab and a
  // cancelled navigation leave the href, and so the drawer, alone.
  const href = useRouterState({ select: (state) => state.location.href });
  const shownHref = useRef(href);
  useEffect(() => {
    if (shownHref.current === href) return;
    shownHref.current = href;
    setOpenMobile(false);
  }, [href, setOpenMobile]);
  // Desktop always shows the icon rail (with its own toggle in the sidebar
  // header, w2/m63), so the content-header trigger is only needed on mobile,
  // where the sidebar is an off-canvas Sheet — show it when that Sheet is
  // closed so there is a way to open it.
  const showTrigger = isMobile && !openMobile;

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
            <GithubIcon />
            {t("common.topbarRepository")}
          </a>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
