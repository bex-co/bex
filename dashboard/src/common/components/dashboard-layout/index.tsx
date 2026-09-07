import { createContext, useContext } from "react";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
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
  // Already inside the persistent root shell → pass the content straight
  // through so we never render a second sidebar/header.
  if (inShell) {
    return <>{children}</>;
  }
  return (
    <InShellContext.Provider value={true}>
      <SidebarProvider>
        <div className="flex h-(--visual-viewport-height,100vh) w-full">
          <DashboardSidebar />
          <main className="flex flex-1 flex-col min-w-0 w-full">
            <DashboardHeader />
            {children}
          </main>
        </div>
      </SidebarProvider>
    </InShellContext.Provider>
  );
}
