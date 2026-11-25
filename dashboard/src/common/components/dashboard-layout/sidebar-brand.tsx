import { Link } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { SidebarTrigger } from "@/common/components/ui/sidebar.tsx";
import { WorkspaceSwitcher } from "@/features/workspaces/components/workspace-switcher";

/**
 * Render parity: the logo + workspace switcher pinned at the very top of the
 * left pane, identical on every page — including a project's own pages,
 * where the rest of the sidebar swaps to the contextual `ProjectSidebar`.
 * The logo always links back to the workspace Overview (Render's own
 * behavior), independent of whatever contextual nav renders below it.
 *
 * The collapse toggle (w2/m63) lives here so it is present in both states: when
 * expanded it sits at the right of the brand row; when collapsed to the icon
 * rail the logo + switcher hide and only the toggle remains (centered),
 * matching Devin's rail. Mobile keeps the full brand (the Sheet has room).
 */
export function SidebarBrand() {
  const { t } = useTranslations();
  return (
    <div className="flex items-center gap-2">
      <div className="flex min-w-0 flex-1 items-center gap-2 group-data-[collapsible=icon]:hidden">
        <Link
          to="/"
          aria-label={t("common.goHome")}
          className="shrink-0 rounded-md ring-offset-sidebar transition-opacity hover:opacity-80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        >
          <img
            src="/logo.webp"
            alt={t("common.appName")}
            className="size-7 rounded-md"
          />
        </Link>
        <div className="min-w-0 flex-1">
          <WorkspaceSwitcher />
        </div>
      </div>
      {/* A 44px target on a touch screen; the 28px button keeps the
          header's 48px row. */}
      <SidebarTrigger className="relative shrink-0 group-data-[collapsible=icon]:mx-auto pointer-coarse:after:absolute pointer-coarse:after:-inset-2 pointer-coarse:after:content-['']" />
    </div>
  );
}
