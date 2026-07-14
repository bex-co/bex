import { Link } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { WorkspaceSwitcher } from "@/features/workspaces/components/workspace-switcher";

/**
 * Render parity: the logo + workspace switcher pinned at the very top of the
 * left pane, identical on every page — including a project's own pages,
 * where the rest of the sidebar swaps to the contextual `ProjectSidebar`.
 * The logo always links back to the workspace Overview (Render's own
 * behavior), independent of whatever contextual nav renders below it.
 */
export function SidebarBrand() {
  const { t } = useTranslations();
  return (
    <div className="flex items-center gap-2">
      <Link
        to="/"
        aria-label={t("common.goHome")}
        className="shrink-0 rounded-md ring-offset-background transition-opacity hover:opacity-80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
      >
        <img
          src="/logo.png"
          alt={t("common.appName")}
          className="size-7 rounded-md"
        />
      </Link>
      <div className="min-w-0 flex-1">
        <WorkspaceSwitcher />
      </div>
    </div>
  );
}
