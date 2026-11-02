import { createContext, useContext } from "react";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { DashboardSidebar } from "./dashboard-sidebar";
import { DashboardHeader } from "./dashboard-header";

/**
 * True once a `DashboardLayout` has mounted the real shell above us. The shell
 * now lives ONCE at the root (`RootComponent`, gated on `staticData.chrome`)
 * and persists across every navigation, so only the content region swaps — no
 * remount of the sidebar/header/provider, and the pending fallback paints
 * inside the shell instead of blanking the whole viewport (the "white flash").
 *
 * Any `DashboardLayout` rendered *inside* a page (the legacy per-page wrapper)
 * therefore reads this context and becomes a pass-through — it renders just its
 * children rather than a second sidebar/header. That keeps the fix correct even
 * while those inner wrappers are being removed, and guarantees a page can never
 * double up the chrome.
 */
const MAIN_CONTENT_ID = "main-content";

const InShellContext = createContext(false);

/**
 * Dashboard layout — the ONE contextual sidebar + Render-style topbar wrapping
 * every authenticated page. The topbar supplies hierarchy, global search,
 * create, help, and account navigation regardless of the active route.
 *
 * There is deliberately no `sidebar` override prop (removed in w5/m64): the
 * rail varies by route *inside* `DashboardSidebar` — `ProjectSidebar` and
 * `ServiceSidebar` replace the nav, the agent-sessions section augments it —
 * so a page never supplies or renders a rail of its own. A page that renders
 * its own `<aside>` produces two side-by-side sidebars, which is what `/agents`
 * shipped before w5/m64; `routes/__tests__/one-rail-invariant.test.ts` guards
 * against the regression.
 */
export function DashboardLayout({ children }: { children?: React.ReactNode }) {
  const inShell = useContext(InShellContext);
  const { t } = useTranslations();
  // Already inside the persistent root shell → pass the content straight
  // through so we never render a second sidebar/header.
  if (inShell) {
    return <>{children}</>;
  }
  return (
    <InShellContext.Provider value={true}>
      <SidebarProvider>
        {/* WCAG 2.4.1 bypass block (w4/460): the first Tab stop, visible on
            focus, jumps past the sidebar and header to the page content. */}
        <a
          href={`#${MAIN_CONTENT_ID}`}
          onClick={(event) => {
            const main = document.getElementById(MAIN_CONTENT_ID);
            if (!main) return;
            event.preventDefault();
            main.focus();
          }}
          className="bg-background text-foreground focus:ring-ring sr-only z-50 rounded-md px-3 py-2 text-sm font-medium shadow focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:ring-2 focus:outline-hidden"
        >
          {t("common.skipToMainContent")}
        </a>
        <div className="flex h-(--visual-viewport-height,100vh) w-full">
          <DashboardSidebar />
          <main
            id={MAIN_CONTENT_ID}
            tabIndex={-1}
            className="flex flex-1 flex-col min-w-0 w-full focus:outline-hidden"
          >
            <DashboardHeader />
            {children}
          </main>
        </div>
      </SidebarProvider>
    </InShellContext.Provider>
  );
}
